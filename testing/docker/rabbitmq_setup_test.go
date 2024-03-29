package docker_test

import (
	"fmt"
	"time"

	"github.com/syncromatics/go-kit/v2/testing/docker"
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

func ExampleSetupRabbitMQWithTimeOut() {
	timeOut := 30 * time.Second
	amqpURL, err := docker.SetupRabbitMQWithTimeOut("test", timeOut)
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
