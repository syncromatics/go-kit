package amqp_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	sut "github.com/syncromatics/go-kit/v2/amqp"
)

func Test_EnsurePublisherIsReady_Successful(t *testing.T) {
	// Arrange
	conn, err := amqp.Dial(amqpURL)
	assert.Nil(t, err)
	defer conn.Close()

	channel, err := conn.Channel()
	assert.Nil(t, err)
	defer channel.Close()

	publisher := sut.NewExchangePublisher(amqpURL)

	// Act
	err = publisher.EnsurePublisherIsReady()

	// Assert
	assert.Nil(t, err)
}

func Test_EnsurePublisherIsReady_BadURL(t *testing.T) {
	// Arrange
	publisher := sut.NewExchangePublisher("amqp://localhost:80")

	// Act
	err := publisher.EnsurePublisherIsReady()

	// Assert
	assert.Error(t, err)
}

func Test_Publish_Successful(t *testing.T) {
	// Arrange
	conn, err := amqp.Dial(amqpURL)
	assert.Nil(t, err)
	defer conn.Close()

	channel, err := conn.Channel()
	assert.Nil(t, err)
	defer channel.Close()

	testQueueName := fmt.Sprintf("test.%v", uuid.NewString())
	_, err = channel.QueueDeclare(testQueueName, false, true, true, false, nil)
	assert.Nil(t, err)

	err = channel.QueueBind(testQueueName, "#", EXCHANGE_NAME, false, nil)
	assert.Nil(t, err)

	actualMessages, err := channel.Consume(testQueueName, uuid.NewString(), true, true, false, false, nil)
	assert.Nil(t, err)

	publisher := sut.NewExchangePublisher(amqpURL)

	err = publisher.EnsurePublisherIsReady()
	assert.Nil(t, err)

	expectedHeaders := amqp.Table{
		"messageType": "Position",
	}
	expectedBody := []byte(`{"VehicleId":1}`)

	// Act
	err = publisher.Publish(EXCHANGE_NAME, map[string]string{
		"messageType": "Position",
	}, expectedBody)

	// Assert
	assert.Nil(t, err)

	select {
	case actual := <-actualMessages:
		assert.Equal(t, expectedHeaders, actual.Headers)
		assert.Equal(t, expectedBody, actual.Body)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "expected to receive message within a timely manner")
	}

}
