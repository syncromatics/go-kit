package docker

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
)

var (
	mssqlImage = "mcr.microsoft.com/mssql/server:2019-GA-ubuntu-16.04"
)

// MSSqlDatabaseSettings is the settings for the database
type MSSqlDatabaseSettings struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     *string
}

func (ds *MSSqlDatabaseSettings) getPort() int {
	if ds.Port != 0 {
		return ds.Port
	}

	return 1433
}

// GetDB will return a database instance from the settings
func (ds *MSSqlDatabaseSettings) GetDB() (*sql.DB, error) {
	query := url.Values{}

	if ds.Name != nil {
		query.Add("database", *ds.Name)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(ds.User, ds.Password),
		Host:     fmt.Sprintf("%s:%d", ds.Host, ds.getPort()),
		RawQuery: query.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, errors.Wrap(err, "open db failed")
	}

	return db, nil
}

// WaitForDatabaseToBeOnline will wait for the database server to be online for the given seconds.
func (ds *MSSqlDatabaseSettings) WaitForDatabaseToBeOnline(secondsToWait int) error {
	db, err := ds.GetDB()
	if err != nil {
		return err
	}

	defer db.Close()

	for i := 0; i < secondsToWait; i++ {
		err = db.Ping()
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return err
}

// DatabaseSetup is the settings for the test database
type DatabaseSetup struct {
	TestName      string
	DatabaseImage *string
	UserName      string
	Password      string
	DatabaseName  *string
}

// SetupMSSqlDatabase sets up a Microsoft SQL Server database
func SetupMSSqlDatabase(setup DatabaseSetup) (*MSSqlDatabaseSettings, error) {
	containerName := fmt.Sprintf("%s_mssql", setup.TestName)

	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	image := mssqlImage
	if setup.DatabaseImage != nil {
		image = *setup.DatabaseImage
	}

	r, err := cli.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}

	_, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	err = r.Close()
	if err != nil {
		return nil, err
	}

	env := []string{
		"ACCEPT_EULA=yes",
	}
	if setup.UserName == "sa" {
		env = append(env, fmt.Sprintf("SA_PASSWORD=%s", setup.Password))
	}

	dbPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}

	config := container.Config{
		Image: image,
		Env:   env,
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"1433/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", dbPort)},
			},
		},
	}
	networkConfig := network.NetworkingConfig{}

	removeContainer(cli, containerName)

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, containerName)
	if err != nil {
		return nil, err
	}

	conChan, errChan := cli.ContainerWait(context.Background(), create.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errChan:
		return nil, err
	case <-conChan:
	}

	err = cli.ContainerStart(context.Background(), create.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, err
	}

	settings, err := waitForDatabaseToBeReady(cli, create.ID, setup, dbPort)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// TeardownMSSqlDatabase tears down the Microsoft SQL Server database
func TeardownMSSqlDatabase(testName string) error {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	removeContainer(cli, fmt.Sprintf("%s_mssql", testName))

	return nil
}

func removeContainer(client *client.Client, name string) {
	client.ContainerRemove(context.Background(), name, types.ContainerRemoveOptions{Force: true})
}

func waitForDatabaseToBeReady(client *client.Client, id string, setup DatabaseSetup, dbPort int) (*MSSqlDatabaseSettings, error) {
	settings := &MSSqlDatabaseSettings{
		Host:     "localhost",
		Port:     dbPort,
		User:     setup.UserName,
		Password: setup.Password,
		Name:     setup.DatabaseName,
	}

	err := settings.WaitForDatabaseToBeOnline(60)
	if err != nil {
		return nil, err
	}

	return settings, nil
}
