package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valB, err := json.Marshal(val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{ContentType: "application/json", Body: valB},
	)
	if err != nil {
		return err
	}
	return nil
}

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

type HandlerOutcome int

const (
	Ack = iota
	NackRequeue
	NackDiscard
)

var outcomeName = map[HandlerOutcome]string{
	Ack:         "ack",
	NackRequeue: "nack-requeue",
	NackDiscard: "nack-discard",
}

func (o HandlerOutcome) String() string {
	return outcomeName[o]
}

func SubscribeJSON[T any](
	conn *amqp.Connection, exchange, queueName, key string, simpleQueueType QueueType, handler func(T) HandlerOutcome,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	deliveryChan, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for m := range deliveryChan {
			var val T
			if err := json.Unmarshal(m.Body, &val); err != nil {
				log.Printf("failed to unmarshal body %v. err: %v\n", m.Body, err)
				continue
			}
			switch handler(val) {
			case Ack:
				log.Println("msg ack")
				m.Ack(false)
			case NackRequeue:
				log.Println("nack requeue")
				m.Nack(false, true)
			case NackDiscard:
				log.Println("nack discard")
				m.Nack(false, false)
			}
		}
	}()

	return nil
}
