package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"github.com/Shopify/sarama"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	v1 "github.com/syncromatics/proto-schema-registry/pkg/proto/schema/registry/v1"
	"google.golang.org/grpc"
)

var (
	zookeeperImage = "docker.io/confluentinc/cp-zookeeper:5.0.0"
	kafkaImage     = "docker.io/confluentinc/cp-kafka:5.0.0"
	registryImage  = "docker.io/syncromatics/proto-schema-registry:v0.7.0"
)

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

// Printf does nothing
func (l *NoOpLogger) Printf(string, ...interface{}) {}

// Panicf will panic
func (l *NoOpLogger) Panicf(msg string, args ...interface{}) {
	log.Panicf(msg, args...)
}

// KafkaSetup is the details about the kafka setup
type KafkaSetup struct {
	ExternalBroker string
	ProtoRegistry  string
}

// SetupKafka sets up the zookeeper and kafka test containers
func SetupKafka(testName string) (*KafkaSetup, error) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	setup := KafkaSetup{}

	err = pullImage(cli, kafkaImage)
	if err != nil {
		return nil, err
	}
	err = pullImage(cli, zookeeperImage)
	if err != nil {
		return nil, err
	}
	err = pullImage(cli, registryImage)
	if err != nil {
		return nil, err
	}

	removeContainers(cli, testName)

	err = createKafkaNetwork(cli, testName)
	if err != nil {
		return nil, err
	}

	zookeeper, err := createZookeeperContainer(cli, testName)
	if err != nil {
		return nil, err
	}

	internalBroker, externalBroker, err := createKafkaContainer(cli, zookeeper, testName)
	if err != nil {
		return nil, err
	}
	setup.ExternalBroker = externalBroker

	err = ensureAdminConnectionToBroker(externalBroker, 60*time.Second)
	if err != nil {
		return nil, err
	}

	protoRegistry, err := createProtoRegistryContainer(cli, testName, internalBroker)
	if err != nil {
		return nil, err
	}
	setup.ProtoRegistry = protoRegistry

	return &setup, nil
}

// TeardownKafka will remove the containers of the test kafka setup
func TeardownKafka(testName string) {
	os.Setenv("DOCKER_API_VERSION", "1.35")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	removeContainers(cli, testName)
}

func pullImage(client *client.Client, image string) error {
	r, err := client.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	// todo: find a way to test body without docker package, because it does not play well with go mod

	return nil
}

func removeContainers(client *client.Client, testName string) {
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_kafka_zk", testName), types.ContainerRemoveOptions{Force: true})
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_kafka_broker", testName), types.ContainerRemoveOptions{Force: true})
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_kafka_pr", testName), types.ContainerRemoveOptions{Force: true})
	client.NetworkRemove(context.Background(), fmt.Sprintf("%s_kafka", testName))
}

func createKafkaNetwork(cli *client.Client, testName string) error {
	networkName := fmt.Sprintf("%s_kafka", testName)
	config := types.NetworkCreate{
		Driver: "bridge",
	}

	_, err := cli.NetworkCreate(context.Background(), networkName, config)
	if err != nil {
		return err
	}

	return nil
}
func createZookeeperContainer(cli *client.Client, testName string) ([]string, error) {
	zkPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}

	config := container.Config{
		Image: zookeeperImage,
		Env: []string{
			"ZOOKEEPER_CLIENT_PORT=2181",
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"2181/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", zkPort)},
			},
		},
	}
	networkConfig := network.NetworkingConfig{}

	hostname := fmt.Sprintf("%s_kafka_zk", testName)
	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, hostname)
	if err != nil {
		return nil, err
	}

	err = cli.NetworkConnect(context.Background(), fmt.Sprintf("%s_kafka", testName), create.ID, nil)
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

	return []string{
		fmt.Sprintf("%s:2181", hostname),
		fmt.Sprintf("localhost:%d", zkPort),
	}, nil
}

func createKafkaContainer(cli *client.Client, zookeeperIP []string, testName string) (string, string, error) {
	kafkaPort, err := freeport.GetFreePort()
	if err != nil {
		return "", "", err
	}

	hostname := fmt.Sprintf("%s_kafka_broker", testName)
	config := container.Config{
		Image: kafkaImage,
		Env: []string{
			fmt.Sprintf("HOST_IP=%s", hostname),
			"KAFKA_NUM_PARTITIONS=10",
			"KAFKA_DEFAULT_REPLICATION_FACTOR=1",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1",
			"KAFKA_REPLICATION_FACTOR=1",
			"KAFKA_BROKER_ID=1",
			fmt.Sprintf("KAFKA_ZOOKEEPER_CONNECT=%s", strings.Join(zookeeperIP, ",")),
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=INT:PLAINTEXT,EXT:PLAINTEXT",
			"KAFKA_LISTENERS=INT://:9090,EXT://:9092",
			fmt.Sprintf("KAFKA_ADVERTISED_LISTENERS=INT://:9090,EXT://localhost:%d", kafkaPort),
			"KAFKA_INTER_BROKER_LISTENER_NAME=INT",
		},
		Entrypoint: nil,
		Cmd: []string{
			"/bin/sh",
			"-c",
			"exec /etc/confluent/docker/run",
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"9092/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", kafkaPort)},
			},
		},
	}
	networkConfig := network.NetworkingConfig{}

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, hostname)
	if err != nil {
		return "", "", err
	}

	err = cli.NetworkConnect(context.Background(), fmt.Sprintf("%s_kafka", testName), create.ID, nil)
	if err != nil {
		return "", "", err
	}

	conChan, errChan := cli.ContainerWait(context.Background(), create.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errChan:
		return "", "", err
	case <-conChan:
	}

	err = cli.ContainerStart(context.Background(), create.ID, types.ContainerStartOptions{})
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s:9090", hostname), fmt.Sprintf("localhost:%d", kafkaPort), nil
}

func createProtoRegistryContainer(cli *client.Client, testName string, brokerIP string) (string, error) {
	registryPort, err := freeport.GetFreePort()
	if err != nil {
		return "", err
	}

	config := container.Config{
		Image: registryImage,
		Env: []string{
			fmt.Sprintf("KAFKA_BROKER=%s", brokerIP),
			"PORT=443",
			"REPLICATION_FACTOR=1",
		},
		ExposedPorts: nat.PortSet{
			"443/tcp": struct{}{},
		},
	}
	hostConfig := container.HostConfig{
		PortBindings: nat.PortMap{
			"443/tcp": []nat.PortBinding{
				{HostPort: fmt.Sprintf("%d/tcp", registryPort)},
			},
		},
	}
	networkConfig := network.NetworkingConfig{}

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, fmt.Sprintf("%s_kafka_pr", testName))
	if err != nil {
		return "", err
	}

	err = cli.NetworkConnect(context.Background(), fmt.Sprintf("%s_kafka", testName), create.ID, nil)
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

	target := fmt.Sprintf("localhost:%d", registryPort)
	con, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		return "", errors.Wrap(err, "failed to dial server")
	}

	client := v1.NewRegistryAPIClient(con)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		_, err = client.Ping(context.Background(), &v1.PingRequest{})
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", errors.Wrap(err, "failed to ping server")
	}

	return target, nil
}

func ensureAdminConnectionToBroker(brokerIP string, duration time.Duration) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_0_0_0

	start := time.Now()
	var lastErr error
	for time.Now().Sub(start) < duration {
		clusterAdmin, err := sarama.NewClusterAdmin([]string{brokerIP}, config)
		if err != nil {
			lastErr = errors.Wrap(err, "failed connecting with sarama")
			continue
		}
		clusterAdmin.Close()
		return nil
	}

	return lastErr
}
