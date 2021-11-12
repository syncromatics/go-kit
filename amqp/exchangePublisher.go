package amqp

import (
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// ExchangePublisher is a service for publishing messages to an exchange
type ExchangePublisher struct {
	amqpURL string

	connection *amqp.Connection
}

// NewExchangePublisher creates a Publisher
func NewExchangePublisher(amqpURL string) *ExchangePublisher {
	return &ExchangePublisher{
		amqpURL: amqpURL,
	}
}

// EnsurePublisherIsReady ensures that the publisher is ready to send messages
func (p *ExchangePublisher) EnsurePublisherIsReady() error {
	var err error
	p.connection, err = amqp.Dial(p.amqpURL)
	if err != nil {
		return errors.Wrap(err, "failed to connect to broker")
	}

	return nil
}

// Publish publishes a message to the given exchange
func (p *ExchangePublisher) Publish(exchangeName string, headers map[string]string, body []byte) error {
	channel, err := p.connection.Channel()
	if err != nil {
		return errors.Wrap(err, "failed to open channel to broker")
	}
	defer channel.Close()

	headersTable := make(amqp.Table)
	for k, v := range headers {
		headersTable[k] = v
	}
	err = channel.Publish(exchangeName, "", false, false, amqp.Publishing{
		Headers: headersTable,
		Body:    body,
	})
	if err != nil {
		return errors.Wrap(err, "failed to publish message")
	}

	return nil
}
