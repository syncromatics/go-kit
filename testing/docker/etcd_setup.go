package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"docker.io/go-docker/api/types/strslice"
)

var (
	etcdImage = "quay.io/coreos/etcd:v3.4.3"
)

// SetupEtcd sets up a etcd database
func SetupEtcd(testName string) (string, error) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	r, err := cli.ImagePull(context.Background(), etcdImage, types.ImagePullOptions{})
	if err != nil {
		return "", err
	}

	_, err = ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	err = r.Close()
	if err != nil {
		return "", err
	}

	config := container.Config{
		Image: etcdImage,
		Cmd: strslice.StrSlice([]string{
			"/usr/local/bin/etcd",
			"-advertise-client-urls",
			"http://0.0.0.0:2379",
			"-listen-client-urls",
			"http://0.0.0.0:2379",
		}),
	}

	hostConfig := container.HostConfig{}

	networkConfig := network.NetworkingConfig{}

	removeEtcdContainer(cli, testName)

	containerName := fmt.Sprintf("%s_etcd_db", testName)
	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, containerName)
	if err != nil {
		return "", err
	}

	conChan, errChan := cli.ContainerWait(context.Background(), create.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errChan:
		return "", err
	case <-conChan:
	}

	err = cli.ContainerStart(context.Background(), create.ID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	db, err := waitForEtcdToBeReady(cli, create.ID)
	if err != nil {
		return "", err
	}

	return db, nil
}

// TeardownEtcdDatabase tears down the etcd db
func TeardownEtcdDatabase(testName string) error {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	removeEtcdContainer(cli, testName)

	return nil
}

func removeEtcdContainer(client *client.Client, testName string) {
	containerName := fmt.Sprintf("%s_etcd_db", testName)
	client.ContainerRemove(context.Background(), containerName, types.ContainerRemoveOptions{Force: true})
}

func waitForEtcdToBeReady(client *client.Client, id string) (string, error) {
	inspect, err := client.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:2379", inspect.NetworkSettings.IPAddress), nil
}
