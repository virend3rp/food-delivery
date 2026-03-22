package rabbitmq

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Connection struct {
	conn *amqp.Connection
	url  string
}

func NewConnection(url string) (*Connection, error) {
	c := &Connection{url: url}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Connection) connect() error {
	var err error
	for i := 0; i < 5; i++ {
		c.conn, err = amqp.Dial(c.url)
		if err == nil {
			break
		}
		log.Printf("[rabbitmq] connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to connect after retries: %w", err)
	}

	// Run topology setup on a temporary channel
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open setup channel: %w", err)
	}
	defer ch.Close()

	return setupTopology(ch)
}

// setupTopology declares the exchange, DLX, and DLQ once on startup.
func setupTopology(ch *amqp.Channel) error {
	// Dead-letter exchange
	if err := ch.ExchangeDeclare(
		"food_delivery.dlx", "fanout", true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare DLX: %w", err)
	}

	// Dead-letter queue
	if _, err := ch.QueueDeclare(
		"food_delivery.dlq", true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare DLQ: %w", err)
	}

	if err := ch.QueueBind("food_delivery.dlq", "", "food_delivery.dlx", false, nil); err != nil {
		return fmt.Errorf("bind DLQ: %w", err)
	}

	// Main topic exchange
	if err := ch.ExchangeDeclare(
		"food_delivery", "topic", true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare main exchange: %w", err)
	}

	log.Println("[rabbitmq] topology ready")
	return nil
}

// NewChannel opens a fresh AMQP channel on this connection.
func (c *Connection) NewChannel() (*amqp.Channel, error) {
	return c.conn.Channel()
}

func (c *Connection) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
