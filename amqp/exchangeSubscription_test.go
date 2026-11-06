package amqp_test

import (
	"context"
	"testing"
	"time"

	sut "github.com/syncromatics/go-kit/v2/amqp"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func Test_EnsureQueueIsReady_Successful(t *testing.T) {
	// Arrange
	conn, err := amqp.Dial(amqpURL)
	assert.Nil(t, err)
	defer conn.Close()

	channel, err := conn.Channel()
	assert.Nil(t, err)
	defer channel.Close()

	exchangeSubscription := sut.NewExchangeSubscription(amqpURL, EXCHANGE_NAME)

	// Act
	err = exchangeSubscription.EnsureExchangeSubscriptionIsReady()

	// Assert
	assert.Nil(t, err)
}

func Test_EnsureQueueIsReady_BadURL(t *testing.T) {
	// Arrange
	exchangeSubscription := sut.NewExchangeSubscription("amqp://localhost:80", EXCHANGE_NAME)

	// Act
	err := exchangeSubscription.EnsureExchangeSubscriptionIsReady()

	// Assert
	assert.Error(t, err)
}

func Test_Consume_Successful(t *testing.T) {
	// Arrange
	conn, err := amqp.Dial(amqpURL)
	assert.Nil(t, err)
	defer conn.Close()

	channel, err := conn.Channel()
	assert.Nil(t, err)
	defer channel.Close()

	exchangeSubscription := sut.NewExchangeSubscription(amqpURL, EXCHANGE_NAME)
	err = exchangeSubscription.EnsureExchangeSubscriptionIsReady()
	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	messages, err := exchangeSubscription.Consume(ctx)
	assert.Nil(t, err)

	// Act
	expectedHeaders := map[string]interface{}{
		"messageType":         "Emergency",
		"otherUnexpectedData": uuid.NewString(),
	}
	expectedBody := []byte(`{"VehicleId":987,"Time":"2030-01-01T00:00:00+00:00"}`)
	err = channel.Publish(EXCHANGE_NAME, "", false, false, amqp.Publishing{
		Headers: expectedHeaders,
		Body:    expectedBody,
	})
	assert.Nil(t, err)

	// Assert & Nack
	select {
	case message, ok := <-messages:
		assert.True(t, ok)
		assert.Equal(t, expectedHeaders, message.Headers)
		assert.Equal(t, expectedBody, message.Body)
		err = message.Nack()
		assert.Nil(t, err)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not receive message in a timely manner")
	}

	// Assert & Ack
	select {
	case messages, ok := <-messages:
		assert.True(t, ok)
		assert.Equal(t, expectedHeaders, messages.Headers)
		assert.Equal(t, expectedBody, messages.Body)
		err = messages.Ack()
		assert.Nil(t, err)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not receive message in a timely manner")
	}

	// Act
	cancel()

	// Assert
	select {
	case _, ok := <-messages:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not close channel in a timely manner")
	}
}

func Test_Consume_HandleConnectionLoss(t *testing.T) {
	// Arrange
	failureTestURL, err := docker.SetupRabbitMQ("failure_test")
	assert.Nil(t, err)
	teardown := func() error {
		return docker.TeardownRabbitMQ("failure_test")
	}

	err = setupExchanges(failureTestURL)
	assert.Nil(t, err)

	exchangeSubscription := sut.NewExchangeSubscription(failureTestURL, EXCHANGE_NAME)
	err = exchangeSubscription.EnsureExchangeSubscriptionIsReady()
	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages, err := exchangeSubscription.Consume(ctx)
	assert.Nil(t, err)

	// Act
	err = teardown()
	assert.Nil(t, err)

	// Assert
	select {
	case _, ok := <-messages:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not close channel in a timely manner")
	}
}

func Test_Consume_HandleContextCancellation(t *testing.T) {
	// Arrange
	exchangeSubscription := sut.NewExchangeSubscription(amqpURL, EXCHANGE_NAME)
	err := exchangeSubscription.EnsureExchangeSubscriptionIsReady()
	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	messages, err := exchangeSubscription.Consume(ctx)
	assert.Nil(t, err)

	// Act
	cancel()

	// Assert
	select {
	case _, ok := <-messages:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not close channel in a timely manner")
	}
}

func Test_Consume_HandleContextCancellationMidProcessing(t *testing.T) {
	// Arrange
	conn, err := amqp.Dial(amqpURL)
	assert.Nil(t, err)
	defer conn.Close()

	channel, err := conn.Channel()
	assert.Nil(t, err)
	defer channel.Close()

	exchangeSubscription := sut.NewExchangeSubscription(amqpURL, EXCHANGE_NAME)
	err = exchangeSubscription.EnsureExchangeSubscriptionIsReady()
	assert.Nil(t, err)

	err = channel.Publish(EXCHANGE_NAME, "", false, false, amqp.Publishing{
		Headers: amqp.Table{
			"messageType": "Emergency",
		},
		Body: []byte(`{"VehicleId":987,"Time":"2030-01-01T00:00:00+00:00"}`),
	})
	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	messages, err := exchangeSubscription.Consume(ctx)
	assert.Nil(t, err)

	// Act
	cancel()

	// Assert
	select {
	case _, ok := <-messages:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not close channel in a timely manner")
	}
}

func Test_ExchangeName_should_return_the_name_of_the_exchange(t *testing.T) {
	// Arrange
	expected := uuid.NewString()
	exchangeSubscription := sut.NewExchangeSubscription("amqp://localhost:80", expected)

	// Act
	actual := exchangeSubscription.ExchangeName()

	// Assert
	assert.Equal(t, expected, actual)
}
