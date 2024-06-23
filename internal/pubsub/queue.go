package pubsub

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueType int

const (
	QueueDurable = iota
	QueueTransient
)

var queueName = map[QueueType]string{
	QueueDurable:   "durable",
	QueueTransient: "transient",
}

func (ss QueueType) String() string {
	return queueName[ss]
}

func DeclareAndBind(
	conn *amqp.Connection, exchange, queueName, key string, simpleQueueType QueueType,
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Failed to open channel:", err)
		return nil, amqp.Queue{}, err
	}

	queue, err := ch.QueueDeclare(
		queueName,
		simpleQueueType == QueueDurable,
		simpleQueueType == QueueTransient,
		simpleQueueType == QueueTransient,
		false,
		amqp.Table{"x-dead-letter-exchange": "peril_dlx"},
	)
	if err != nil {
		fmt.Println("Queue declaration failed:", err)
		return nil, amqp.Queue{}, err
	}

	if err := ch.QueueBind(queue.Name, key, exchange, false, nil); err != nil {
		fmt.Println("Queue binding failed:", err)
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}
