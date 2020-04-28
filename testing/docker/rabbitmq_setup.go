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
)

var (
	rabbitMqImage = "rabbitmq:3.7.7-management"
)

// SetupRabbitMQ sets up a RabbitMQ broker
func SetupRabbitMQ(testName string) (string, error) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	r, err := cli.ImagePull(context.Background(), rabbitMqImage, types.ImagePullOptions{})
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
		Image: rabbitMqImage,
	}
	hostConfig := container.HostConfig{}
	networkConfig := network.NetworkingConfig{}

	removeRabbitMQContainer(cli, testName)

	containerName := fmt.Sprintf("%s_rabbitmq", testName)
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

	url, err := waitForRabbitMQToBeReady(cli, create.ID)
	if err != nil {
		return "", err
	}

	return url, nil
}

// TeardownRabbitMQ tears down the RabbitMQ broker
func TeardownRabbitMQ(testName string) error {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	removeRabbitMQContainer(cli, testName)
	return nil
}

func removeRabbitMQContainer(client *client.Client, testName string) {
	containerName := fmt.Sprintf("%s_rabbitmq", testName)
	client.ContainerRemove(context.Background(), containerName, types.ContainerRemoveOptions{Force: true})
}

func waitForRabbitMQToBeReady(client *client.Client, id string) (string, error) {
	inspect, err := client.ContainerInspect(context.Background(), id)
	if err != nil {
		return "", err
	}

	ip := inspect.NetworkSettings.IPAddress
	url := fmt.Sprintf("amqp://guest:guest@%s:5672", ip)

	return url, nil
}
