package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Println("Connection established")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	gamelogic.PrintServerHelp()

	err = pubsub.SubscribeGob(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		fmt.Sprintf("%s.*", routing.GameLogSlug),
		pubsub.QueueDurable,
		handlerLogs,
	)
	if err != nil {
		log.Fatal(err)
	}

	for loop := true; loop; {
		inputs := gamelogic.GetInput()
		if len(inputs) == 0 {
			continue
		}
		switch inputs[0] {
		case "pause":
			log.Println("sending pause message")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				log.Fatal(err)
			}
		case "resume":
			log.Println("sending resume message")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				log.Fatal(err)
			}
		case "help":
			gamelogic.PrintServerHelp()
		case "quit":
			log.Println("shutting down")
			loop = false
		default:
			log.Println("unknown command")
		}
	}
}

func handlerLogs(gl routing.GameLog) pubsub.HandlerOutcome {
	defer fmt.Print("> ")
	if err := gamelogic.WriteLog(gl); err != nil {
		log.Printf("log handler error: %v\n", err)
		return pubsub.NackRequeue
	}
	return pubsub.Ack
}
