package docker

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"github.com/pkg/errors"
)

var (
	databaseImage = "mcr.microsoft.com/mssql/server:2019-GA-ubuntu-16.04"
)

// MSSqlDatabaseSettings is the settings for the database
type MSSqlDatabaseSettings struct {
	Host     string
	User     string
	Password string
	Name     *string
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
		Host:     fmt.Sprintf("%s:%d", ds.Host, 1433),
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

// SetupAdminDatabase will setup a test database
func SetupAdminDatabase(setup DatabaseSetup) (*MSSqlDatabaseSettings, error) {
	containerName := fmt.Sprintf("%s_database_test", setup.TestName)

	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	image := databaseImage
	if setup.DatabaseImage != nil {
		image = *setup.DatabaseImage
	}

	config := container.Config{
		Image: image,
	}

	hostConfig := container.HostConfig{}

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

	settings, err := waitForDatabaseToBeReady(cli, create.ID, setup)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// TeardownDatabase will teardown the test database
func TeardownDatabase(testName string) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	removeContainer(cli, fmt.Sprintf("%s_database_test", testName))
}

func removeContainer(client *client.Client, name string) {
	client.ContainerRemove(context.Background(), name, types.ContainerRemoveOptions{Force: true})
}

func waitForDatabaseToBeReady(client *client.Client, id string, setup DatabaseSetup) (*MSSqlDatabaseSettings, error) {
	inspect, err := client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	settings := &MSSqlDatabaseSettings{
		Host:     inspect.NetworkSettings.IPAddress,
		User:     setup.UserName,
		Password: setup.Password,
		Name:     setup.DatabaseName,
	}

	err = waitForDatabaseToBeOnline(60, settings)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func waitForDatabaseToBeOnline(secondsToWait int, settings *MSSqlDatabaseSettings) error {
	query := url.Values{}
	query.Add("app name", "util")

	if settings.Name != nil {
		query.Add("database", *settings.Name)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(settings.User, settings.Password),
		Host:     fmt.Sprintf("%s:%d", settings.Host, 1433),
		RawQuery: query.Encode(),
	}

	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return errors.Wrap(err, "opening sql failed")
	}

	for i := 0; i < secondsToWait; i++ {
		err = db.Ping()
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return err
}
