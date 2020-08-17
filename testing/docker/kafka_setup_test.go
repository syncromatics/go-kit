package docker_test

import (
	"fmt"

	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func ExampleSetupKafka() {
	setup, err := docker.SetupKafka("test")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Kafka broker and proto registry hosts are set: %t", setup.ExternalBroker != "" && setup.ProtoRegistry != "")

	docker.TeardownKafka("test")
	// Output: Kafka broker and proto registry hosts are set: true
}
