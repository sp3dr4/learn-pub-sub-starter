package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

func publish[T any](ch *amqp.Channel, exchange, key string, val T, marshaller func(T) ([]byte, error), contentType string) error {
	valBytes, err := marshaller(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{ContentType: contentType, Body: valBytes},
	)
	if err != nil {
		return err
	}
	return nil
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonMarshaller := func(T) ([]byte, error) {
		return json.Marshal(val)
	}
	return publish(ch, exchange, key, val, jsonMarshaller, "application/json")
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	gobMarshaller := func(T) ([]byte, error) {
		var network bytes.Buffer
		enc := gob.NewEncoder(&network)
		if err := enc.Encode(val); err != nil {
			return nil, err
		}
		return network.Bytes(), nil
	}
	return publish(ch, exchange, key, val, gobMarshaller, "application/gob")
}
