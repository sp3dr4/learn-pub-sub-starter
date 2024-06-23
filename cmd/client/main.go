package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

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

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatal(err)
	}

	state := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, username),
		routing.PauseKey,
		pubsub.QueueTransient,
		handlerPause(ch, state),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username),
		fmt.Sprintf("%s.*", routing.ArmyMovesPrefix),
		pubsub.QueueTransient,
		handlerMove(ch, state),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix),
		pubsub.QueueDurable,
		handlerWar(ch, state),
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
		case "spawn":
			if err := state.CommandSpawn(inputs); err != nil {
				log.Printf("spawn error: %v\n", err)
			}
		case "move":
			move, err := state.CommandMove(inputs)
			if err != nil {
				log.Printf("move error: %v\n", err)
			}
			key := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username)
			if err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, key, move); err != nil {
				log.Printf("publish move error: %v\n", err)
				continue
			}
			log.Printf("move published to %s\n", key)
		case "status":
			state.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(inputs) < 2 {
				log.Println("spam command needs an additional N argument")
				continue
			}
			spamN, err := strconv.Atoi(inputs[1])
			if err != nil {
				log.Printf("invalid spam amount %v: %v\n", inputs[1], err)
				continue
			}
			key := fmt.Sprintf("%s.%s", routing.GameLogSlug, username)
			for range spamN {
				gamelog := routing.GameLog{CurrentTime: time.Now(), Username: state.GetUsername(), Message: gamelogic.GetMaliciousLog()}
				if err = pubsub.PublishGob(ch, routing.ExchangePerilTopic, key, gamelog); err != nil {
					log.Printf("publish spam error: %v\n", err)
					continue
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			loop = false
		default:
			log.Println("unknown command")
		}
	}
}

func handlerPause(ch *amqp.Channel, gs *gamelogic.GameState) func(routing.PlayingState) pubsub.HandlerOutcome {
	return func(ps routing.PlayingState) pubsub.HandlerOutcome {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(ch *amqp.Channel, gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.HandlerOutcome {
	return func(mv gamelogic.ArmyMove) pubsub.HandlerOutcome {
		defer fmt.Print("> ")
		switch gs.HandleMove(mv) {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			key := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.GetUsername())
			recognition := gamelogic.RecognitionOfWar{Attacker: mv.Player, Defender: gs.Player}
			if err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, key, recognition); err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(ch *amqp.Channel, gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.HandlerOutcome {
	return func(rw gamelogic.RecognitionOfWar) pubsub.HandlerOutcome {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)

		var ackNack pubsub.HandlerOutcome
		logMsg := ""
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			ackNack = pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			ackNack = pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon, gamelogic.WarOutcomeDraw:
			ackNack = pubsub.Ack

			logMsg = fmt.Sprintf("%s won a war against %s", winner, loser)
			if outcome == gamelogic.WarOutcomeDraw {
				logMsg = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			}
		default:
			log.Printf("unknown war outcome %v\n\n", outcome)
			ackNack = pubsub.NackDiscard
		}

		if logMsg != "" {
			gamelog := routing.GameLog{CurrentTime: time.Now(), Username: rw.Attacker.Username, Message: logMsg}
			logRoutingKey := fmt.Sprintf("%s.%s", routing.GameLogSlug, rw.Attacker.Username)
			if err := pubsub.PublishGob(ch, routing.ExchangePerilTopic, logRoutingKey, gamelog); err != nil {
				return pubsub.NackRequeue
			}
		}

		return ackNack
	}
}
