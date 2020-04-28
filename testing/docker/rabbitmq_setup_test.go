package docker_test

import (
	"fmt"

	"github.com/syncromatics/go-kit/testing/docker"
)

func ExampleSetupRabbitMQ() {
	amqpURL, err := docker.SetupRabbitMQ("test")
	if err != nil {
		panic(err)
	}

	fmt.Printf("amqpURL is set: %t\n", amqpURL != "")
	/*
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		panic(err)
	}

	// snip

	err = conn.Close()
	if err != nil {
		panic(err)
	}
	*/
	
	docker.TeardownRabbitMQ("test")
	// Output: amqpURL is set: true
}