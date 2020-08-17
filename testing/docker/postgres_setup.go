package docker

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
	"github.com/syncromatics/go-kit/v2/database"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
)

var (
	postgresImage = "postgres:10.2"
)

// SetupPostgresDatabase sets up a postgres database
func SetupPostgresDatabase(testName string) (*sql.DB, *database.PostgresDatabaseSettings, error) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, nil, err
	}

	r, err := cli.ImagePull(context.Background(), postgresImage, types.ImagePullOptions{})
	if err != nil {
		return nil, nil, err
	}

	_, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}

	err = r.Close()
	if err != nil {
		return nil, nil, err
	}

	dbPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, nil, err
	}

	config := container.Config{
		Image: postgresImage,
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=test",
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", dbPort)},
			},
		},
	}
	networkConfig := network.NetworkingConfig{}

	removePostgresContainer(cli, testName)

	containerName := fmt.Sprintf("%s_postgres_db", testName)
	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, containerName)
	if err != nil {
		return nil, nil, err
	}

	conChan, errChan := cli.ContainerWait(context.Background(), create.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errChan:
		return nil, nil, err
	case <-conChan:
	}

	err = cli.ContainerStart(context.Background(), create.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, nil, err
	}

	db, settings, err := waitForPostgresToBeReady(dbPort)
	if err != nil {
		return nil, nil, err
	}

	return db, settings, nil
}

// TeardownPostgresDatabase tears down the postgres db
func TeardownPostgresDatabase(testName string) error {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	removePostgresContainer(cli, testName)

	return nil
}

func removePostgresContainer(client *client.Client, testName string) {
	containerName := fmt.Sprintf("%s_postgres_db", testName)
	client.ContainerRemove(context.Background(), containerName, types.ContainerRemoveOptions{Force: true})
}

func waitForPostgresToBeReady(dbPort int) (*sql.DB, *database.PostgresDatabaseSettings, error) {
	settings := database.PostgresDatabaseSettings{
		Host:     "localhost",
		Port:     dbPort,
		User:     "postgres",
		Password: "postgres",
		Name:     "test",
	}

	err := settings.WaitForDatabaseToBeOnline(60)
	if err != nil {
		return nil, nil, err
	}

	db, err := settings.EnsureDatabaseExistsAndGetConnection()
	if err != nil {
		return nil, nil, err
	}

	return db, &settings, nil
}
