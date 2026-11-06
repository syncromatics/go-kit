package amqp

import (
	"context"
	"fmt"

	uuid "github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/streadway/amqp"
	"github.com/syncromatics/go-kit/v2/log"
)

// ExchangeSubscription is a service for subscribing to an AMQP exchange
type ExchangeSubscription struct {
	amqpURL      string
	queueName    string
	exchangeName string

	connection *amqp.Connection

	activeConsumers  prometheus.Gauge
	messagesConsumed prometheus.Counter
	messagesAcked    prometheus.Counter
	messagesNacked   prometheus.Counter
	messagesRejected prometheus.Counter
}

// NewExchangeSubscription creates a new ExchangeSubscription
func NewExchangeSubscription(amqpURL string, exchangeName string) *ExchangeSubscription {
	queueName := fmt.Sprintf("%s.%s", exchangeName, uuid.New())

	labels := prometheus.Labels{
		"amqp_queue":    queueName,
		"amqp_exchange": exchangeName,
	}

	activeConsumers := promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "amqp_consumers_total",
		Help:        "The total number of consumers connected to the queue that is subscribed to the exchange",
		ConstLabels: labels,
	})

	messagesConsumed := promauto.NewCounter(prometheus.CounterOpts{
		Name:        "amqp_messages_recv_total",
		Help:        "The total number of received messages",
		ConstLabels: labels,
	})

	messagesAcked := promauto.NewCounter(prometheus.CounterOpts{
		Name:        "amqp_messages_ack_total",
		Help:        "The total number of acknowledged messages",
		ConstLabels: labels,
	})

	messagesNacked := promauto.NewCounter(prometheus.CounterOpts{
		Name:        "amqp_messages_nack_total",
		Help:        "The total number of negatively acknowledged messages",
		ConstLabels: labels,
	})

	messagesRejected := promauto.NewCounter(prometheus.CounterOpts{
		Name:        "amqp_messages_reject_total",
		Help:        "The total number of rejected messages",
		ConstLabels: labels,
	})

	return &ExchangeSubscription{
		amqpURL:      amqpURL,
		queueName:    queueName,
		exchangeName: exchangeName,

		activeConsumers:  activeConsumers,
		messagesConsumed: messagesConsumed,
		messagesAcked:    messagesAcked,
		messagesNacked:   messagesNacked,
		messagesRejected: messagesRejected,
	}
}

// EnsureExchangeSubscriptionIsReady ensures that the necessary transient queue exists and is bound to the exchange
func (es *ExchangeSubscription) EnsureExchangeSubscriptionIsReady() error {
	var err error
	es.connection, err = amqp.Dial(es.amqpURL)
	if err != nil {
		return errors.Wrap(err, "failed to connect to broker")
	}

	channel, err := es.connection.Channel()
	if err != nil {
		return errors.Wrap(err, "failed to open channel to broker")
	}
	defer channel.Close()

	_, err = channel.QueueDeclare(es.queueName, false, true, true, false, nil)
	if err != nil {
		return errors.Wrap(err, "failed to declare queue")
	}

	err = channel.QueueBind(es.queueName, "#", es.exchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "failed to bind queue to exchange")
	}

	return nil
}

// Message represents a message in-flight from an AMQP broker
type Message struct {
	// Headers are the collection of metadata passed along with the Body
	Headers map[string]interface{}
	// Body is the unmodified byte array containing the message
	Body []byte
	// Ack acknowledges the successful processing of the message
	Ack func() error
	// Nack acknowledges the failed processing of the message and instructs the message to be requeued
	Nack func() error
}

// Consume starts consuming messages
//
// Any messages that are not explicitly Acked or Nacked by this consumer before the connection is terminated will be automatically requeued.
func (es *ExchangeSubscription) Consume(outerCtx context.Context) (<-chan *Message, error) {
	channel, err := es.connection.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "failed to open channel for consumer")
	}

	consumer := fmt.Sprintf("%s.consumer", es.queueName)
	rawMessages, err := channel.Consume(es.queueName, consumer, false, true, false, false, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start consuming messages from queue")
	}

	ctx, cancel := context.WithCancel(outerCtx)
	messages := make(chan *Message)
	go func() {
		es.activeConsumers.Inc()
		defer es.activeConsumers.Dec()
		for {
			select {
			case msg, ok := <-rawMessages:
				if !ok {
					cancel()
					continue
				}

				es.messagesConsumed.Inc()

				message := &Message{
					Headers: msg.Headers,
					Body:    msg.Body,
					Ack: func() error {
						es.messagesAcked.Inc()
						return msg.Ack(false)
					},
					Nack: func() error {
						es.messagesNacked.Inc()
						return msg.Nack(false, true)
					},
				}

				select {
				case messages <- message:
				case <-ctx.Done():
					err = message.Nack()
					if err != nil {
						log.Warn("failed to nack in-flight message",
							"err", err,
							"consumer", consumer,
						)
					}
				}
			case <-ctx.Done():
				close(messages)

				err := channel.Cancel(consumer, false)
				if err != nil {
					log.Error("failed to cancel consumer",
						"err", err,
						"consumer", consumer,
					)
				}

				err = channel.Close()
				if err != nil {
					log.Error("failed to close channel for consumer",
						"err", err,
						"consumer", consumer,
					)
				}
				return
			}
		}
	}()

	return messages, nil
}

// ExchangeName is the name of the exchange to which this is subscribed
func (es *ExchangeSubscription) ExchangeName() string {
	return es.exchangeName
}
