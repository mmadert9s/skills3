package main

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func publishObjectUploadedMessage(conn *amqp.Connection, objectUrl string, lastInsertId int) error {
	log.Printf("Publishing message for object %s to mq...", objectUrl)

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	messageBody := fmt.Sprintf(`{"url":"%s","id":"%d"}`, objectUrl, lastInsertId)

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         []byte(messageBody),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx, exchangeName, queueName, true, false, msg)
	if err != nil {
		return err
	}

	log.Printf("Sent message for object %s", objectUrl)

	return nil
}

func initMQ() *amqp.Connection {
	log.Print("Initializing mq...")

	var err error
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:5672/", mqUser, mqPassword, mqHost)) // TODO: connection might break, and must be recovered or lead to pod restart.
	if err != nil {
		log.Fatalf("unexpected error while opening connection to mq: %v", err)
	}

	log.Print("Opened mq connection")

	// Open a channel to create the exchange, queue, and binding
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("unexpected error while opening channel to mq: %v", err)
	}
	defer ch.Close()

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

	return conn
}
