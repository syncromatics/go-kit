package docker

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
)

var (
	rabbitMqImage = "rabbitmq:3.7.7-management"
)

func SetupRabbitMQ(testName string) (string, error) {
	return SetupRabbitMQWithTimeOut(testName, 30*time.Second)
}

// SetupRabbitMQ sets up a RabbitMQ broker
func SetupRabbitMQWithTimeOut(testName string, timeOut time.Duration) (string, error) {
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

	ports, err := freeport.GetFreePorts(2)
	if err != nil {
		return "", err
	}

	amqpPort, managementPort := ports[0], ports[1]

	// Taken from https://github.com/docker-library/healthcheck/blob/master/rabbitmq/docker-healthcheck
	healthcheckScript := `rabbitmqctl eval '
{ true, rabbit_app_booted_and_running } = { rabbit:is_booted(node()), rabbit_app_booted_and_running },
{ [], no_alarms } = { rabbit:alarms(), no_alarms },
[] /= rabbit_networking:active_listeners(),
rabbitmq_node_is_healthy.
' || exit 1`
	config := container.Config{
		Image: rabbitMqImage,
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD-SHELL", healthcheckScript},
			Interval: 1 * time.Second,
			Retries:  30,
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"5672/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", amqpPort)},
			},
			"15672/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", managementPort)},
			},
		},
	}
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

	url, err := waitForRabbitMQToBeReady(cli, create.ID, amqpPort, timeOut)
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

func waitForRabbitMQToBeReady(client *client.Client, id string, amqpPort int, timeOut time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)

	for {
		select {
		case <-ctx.Done():
			cancel()
			return "", errors.New("failed to start container in a timely manner")
		case <-time.After(1 * time.Second):
			inspect, err := client.ContainerInspect(ctx, id)
			if err != nil {
				continue
			}

			if inspect.State.Health.Status == "healthy" {
				cancel()
				return fmt.Sprintf("amqp://guest:guest@localhost:%d", amqpPort), nil
			}
		}
	}
}
