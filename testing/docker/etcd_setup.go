package docker

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"docker.io/go-docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
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

	ports, err := freeport.GetFreePorts(2)
	if err != nil {
		return "", err
	}

	grpcPort, metricsPort := ports[0], ports[1]

	config := container.Config{
		Image: etcdImage,
		Cmd: strslice.StrSlice([]string{
			"/usr/local/bin/etcd",
			"-advertise-client-urls",
			"http://0.0.0.0:2379",
			"-listen-client-urls",
			"http://0.0.0.0:2379",
			"--listen-metrics-urls",
			"http://0.0.0.0:4001",
		}),
		ExposedPorts: nat.PortSet{
			"2379/tcp": struct{}{},
			"4001/tcp": struct{}{},
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"2379/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", grpcPort)},
			},
			"4001/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", metricsPort)},
			},
		},
	}
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

	db, err := waitForEtcdToBeReady(cli, create.ID, grpcPort, metricsPort)
	if err != nil {
		return "", err
	}

	return db, nil
}

// TeardownEtcd tears down the etcd db
func TeardownEtcd(testName string) error {
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

func waitForEtcdToBeReady(client *client.Client, id string, grpcPort, metricsPort int) (string, error) {
	healthURL := fmt.Sprintf("http://localhost:%d/health", metricsPort)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	for {
		select {
		case <-ctx.Done():
			cancel()
			return "", errors.New("failed to start container in a timely manner")
		case <-time.After(1*time.Second):
			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				continue
			}
			
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				continue
			}

			if response.StatusCode == http.StatusOK {
				cancel()
				return fmt.Sprintf("localhost:%d", grpcPort), nil
			}
		}
	}
}
