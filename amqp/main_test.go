package amqp_test

import (
	"os"
	"testing"

	"github.com/streadway/amqp"
	"github.com/syncromatics/go-kit/v2/testing/docker"
)

const (
	EXCHANGE_NAME = "subject_under_test.exchange.name"
)

var (
	amqpURL string
)

func TestMain(m *testing.M) {
	teardowns := setup()

	result := m.Run()

	teardown(teardowns)

	os.Exit(result)
}

func setup() []func() error {
	var (
		err       error
		teardowns []func() error
	)
	amqpURL, err = docker.SetupRabbitMQ("amqp")
	if err != nil {
		panic(err)
	}
	teardowns = append(teardowns, func() error {
		return docker.TeardownRabbitMQ("amqp")
	})

	err = setupExchanges(amqpURL)
	if err != nil {
		panic(err)
	}

	return teardowns
}

func teardown(teardowns []func() error) {
	for _, f := range teardowns {
		err := f()
		if err != nil {
			panic(err)
		}
	}
}

func setupExchanges(url string) error {
	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	topicExchangeNames := []string{
		EXCHANGE_NAME,
	}
	for _, exchangeName := range topicExchangeNames {
		err = channel.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
