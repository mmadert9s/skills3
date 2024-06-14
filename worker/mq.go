package main

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func initConsumer(ch *amqp.Channel) <-chan amqp.Delivery {
	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("unexpected error while registering consumer: %v", err)
	}

	return msgs
}

func initMQ() (*amqp.Connection, *amqp.Channel) {
	log.Print("Initializing mq...")

	var err error
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:5672/", mqUser, mqPassword, mqHost)) // TODO: connection might break, and must be recovered or lead to pod restart.
	if err != nil {
		log.Fatalf("unexpected error while opening connection to mq: %v", err)
	}
	log.Print("Opened mq connection")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("unexpected error while opening channel to mq: %v", err)
	}
	log.Print("Opened mq channel")

	err = ch.ExchangeDeclare(
		exchangeName,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		amqp.Table{},
	)
	if err != nil {
		log.Fatalf("unexpected error while creating exchange: %v", err)
	}
	log.Printf("Declared exchange %s", exchangeName)

	_, err = ch.QueueDeclare(
		queueName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("unexpected error while creating queue: %v", err)
	}
	log.Printf("Declared queue %s", queueName)

	err = ch.QueueBind(
		queueName,
		queueName,
		exchangeName,
		false,
		amqp.Table{},
	)
	if err != nil {
		log.Fatalf("unexpected error while binding queue: %v", err)
	}
	log.Printf("Bound queue %s to exchange %s", queueName, exchangeName)

	log.Print("Initialized mq")

	return conn, ch
}
