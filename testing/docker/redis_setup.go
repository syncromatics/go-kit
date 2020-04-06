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
	goredis "github.com/go-redis/redis"
	"github.com/syncromatics/go-kit/redis"
)

var (
	redisImage = "redis:5.0"
)

// SetupRedis sets up a redis store
func SetupRedis(testName string) (*goredis.Options, error) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	r, err := cli.ImagePull(context.Background(), redisImage, types.ImagePullOptions{})
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

	config := container.Config{
		Image: postgresImage,
	}
	hostConfig := container.HostConfig{}
	networkConfig := network.NetworkingConfig{}

	removeRedisContainer(cli, testName)

	containerName := getContainerName(testName)
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

	opts, err := waitForRedisToBeReady(cli, create.ID)
	if err != nil {
		return nil, err
	}

	return opts, nil
}

// TeardownRedis tears down the redis store
func TeardownRedis(testName string) error {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	removeRedisContainer(cli, testName)
	return nil
}

func removeRedisContainer(client *client.Client, testName string) {
	containerName := getContainerName(testName)
	client.ContainerRemove(context.Background(), containerName, types.ContainerRemoveOptions{Force: true})
}

func getContainerName(testName string) string {
	return fmt.Sprintf("%s_redis", testName)
}

func waitForRedisToBeReady(client *client.Client, id string) (*goredis.Options, error) {
	inspect, err := client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	ip := inspect.NetworkSettings.IPAddress
	url := fmt.Sprintf("redis://%s:6379", ip)

	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	err = redis.WaitForRedisToBeOnline(opts, 60)
	if err != nil {
		return nil, err
	}

	return opts, nil
}
