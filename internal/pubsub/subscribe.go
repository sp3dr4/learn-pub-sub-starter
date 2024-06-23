package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType,
	handler func(T) HandlerOutcome,
	unmarshaller func([]byte) (T, error),
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	if err = ch.Qos(10, 0, false); err != nil {
		return err
	}
	deliveryChan, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for m := range deliveryChan {
			val, err := unmarshaller(m.Body)
			if err != nil {
				log.Printf("failed to unmarshal body %v. err: %v\n", m.Body, err)
				continue
			}

			switch handler(val) {
			case Ack:
				m.Ack(false)
			case NackRequeue:
				m.Nack(false, true)
			case NackDiscard:
				m.Nack(false, false)
			}
		}
	}()

	return nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection, exchange, queueName, key string, simpleQueueType QueueType, handler func(T) HandlerOutcome,
) error {
	unmarshaller := func(raw []byte) (T, error) {
		var val T
		if err := json.Unmarshal(raw, &val); err != nil {
			return val, err
		}
		return val, nil
	}
	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller)
}

func SubscribeGob[T any](
	conn *amqp.Connection, exchange, queueName, key string, simpleQueueType QueueType, handler func(T) HandlerOutcome,
) error {
	unmarshaller := func(raw []byte) (T, error) {
		dec := gob.NewDecoder(bytes.NewBuffer(raw))
		var val T
		if err := dec.Decode(&val); err != nil {
			return val, err
		}
		return val, nil
	}
	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller)
}
