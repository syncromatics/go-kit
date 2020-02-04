package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	client "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/network"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	v1 "github.com/syncromatics/proto-schema-registry/pkg/proto/schema/registry/v1"
	kazoo "github.com/wvanbergen/kazoo-go"
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
	ZookeeperIP     string
	ProtoRegistryIP string
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

	zookeeperIP, err := createZookeeperContainer(cli, testName)
	if err != nil {
		return nil, err
	}
	setup.ZookeeperIP = zookeeperIP

	brokerIP, err := createKafkaContainer(cli, setup.ZookeeperIP, testName)
	if err != nil {
		return nil, err
	}

	_, err = waitForBroker(setup.ZookeeperIP, 60*time.Second)
	if err != nil {
		return nil, err
	}

	protoIP, err := createProtoRegistryContainer(cli, testName, brokerIP)
	if err != nil {
		return nil, err
	}
	setup.ProtoRegistryIP = protoIP

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
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_zookeeper_test", testName), types.ContainerRemoveOptions{Force: true})
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_kafka_test", testName), types.ContainerRemoveOptions{Force: true})
	client.ContainerRemove(context.Background(), fmt.Sprintf("%s_proto_registry_test", testName), types.ContainerRemoveOptions{Force: true})
}

func createZookeeperContainer(cli *client.Client, testName string) (string, error) {
	config := container.Config{
		Image: zookeeperImage,
		Env: []string{
			"ZOOKEEPER_CLIENT_PORT=2181",
		}}

	hostConfig := container.HostConfig{}

	networkConfig := network.NetworkingConfig{}

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, fmt.Sprintf("%s_zookeeper_test", testName))
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

	inspect, err := cli.ContainerInspect(context.Background(), create.ID)
	if err != nil {
		return "", err
	}

	return inspect.NetworkSettings.IPAddress, nil
}

func createKafkaContainer(cli *client.Client, zookeeperIP string, testName string) (string, error) {
	config := container.Config{
		Image: kafkaImage,
		Env: []string{
			"HOST_IP=kafka",
			"KAFKA_NUM_PARTITIONS=10",
			"KAFKA_DEFAULT_REPLICATION_FACTOR=1",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1",
			"KAFKA_REPLICATION_FACTOR=1",
			"KAFKA_BROKER_ID=1",
			fmt.Sprintf("KAFKA_ZOOKEEPER_CONNECT=%s", zookeeperIP),
		},
		Entrypoint: nil,
		Cmd: []string{
			"/bin/sh",
			"-c",
			"export KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://$(hostname -i):9092 && exec /etc/confluent/docker/run",
		},
	}

	hostConfig := container.HostConfig{}

	networkConfig := network.NetworkingConfig{}

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, fmt.Sprintf("%s_kafka_test", testName))
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

	inspect, err := cli.ContainerInspect(context.Background(), create.ID)
	if err != nil {
		return "", err
	}

	return inspect.NetworkSettings.IPAddress, nil
}

func createProtoRegistryContainer(cli *client.Client, testName string, brokerIP string) (string, error) {
	config := container.Config{
		Image: registryImage,
		Env: []string{
			fmt.Sprintf("KAFKA_BROKER=%s:9092", brokerIP),
			"PORT=443",
			"REPLICATION_FACTOR=1",
		},
	}

	hostConfig := container.HostConfig{}

	networkConfig := network.NetworkingConfig{}

	create, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networkConfig, fmt.Sprintf("%s_proto_registry_test", testName))
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

	inspect, err := cli.ContainerInspect(context.Background(), create.ID)
	if err != nil {
		return "", err
	}

	con, err := grpc.Dial(fmt.Sprintf("%s:443", inspect.NetworkSettings.IPAddress), grpc.WithInsecure())
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

	return inspect.NetworkSettings.IPAddress, nil
}

func waitForBroker(zookeeperHost string, duration time.Duration) ([]string, error) {
	start := time.Now()
	for time.Now().Sub(start) < duration {
		var kz *kazoo.Kazoo
		logger := NoOpLogger{}
		kzConfig := kazoo.NewConfig()
		kzConfig.Logger = &logger

		kz, err := kazoo.NewKazoo([]string{zookeeperHost}, kzConfig)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		brokersMap, err := kz.Brokers()
		if err != nil {
			kz.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		if len(brokersMap) > 0 {
			kz.Close()
			brokers := []string{}
			for _, broker := range brokersMap {
				brokers = append(brokers, broker)
			}

			err = ensureAdminConnectionToBroker(brokers, duration)
			if err != nil {
				return nil, err
			}
			return brokers, nil
		}

		time.Sleep(1 * time.Second)

		kz.Close()
	}

	return nil, fmt.Errorf("cannot get brokers")
}

func ensureAdminConnectionToBroker(brokers []string, duration time.Duration) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_0_0_0

	start := time.Now()
	var lastErr error
	for time.Now().Sub(start) < duration {
		clusterAdmin, err := sarama.NewClusterAdmin(brokers, config)
		if err != nil {
			lastErr = errors.Wrap(err, "failed connecting with sarama")
			continue
		}
		clusterAdmin.Close()
		return nil
	}

	return lastErr
}
