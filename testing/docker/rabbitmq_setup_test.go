package docker_test

import (
	"fmt"
	"time"

	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func ExampleSetupRabbitMQ() {
	timeOut := 90 * time.Second
	amqpURL, err := docker.SetupRabbitMQ("test", timeOut)
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
