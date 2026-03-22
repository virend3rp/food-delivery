package rabbitmq

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type HandlerFunc func(ctx context.Context, body []byte) error

type Consumer struct {
	channel *amqp.Channel
}

func NewConsumer(conn *Connection) (*Consumer, error) {
	ch, err := conn.NewChannel()
	if err != nil {
		return nil, fmt.Errorf("consumer: open channel: %w", err)
	}
	return &Consumer{channel: ch}, nil
}

// DeclareQueue creates a durable queue with a DLX and binds it to the given routing keys.
func (c *Consumer) DeclareQueue(name string, bindingKeys ...string) error {
	args := amqp.Table{
		"x-dead-letter-exchange": "food_delivery.dlx",
	}

	if _, err := c.channel.QueueDeclare(name, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare queue %s: %w", name, err)
	}

	for _, key := range bindingKeys {
		if err := c.channel.QueueBind(name, key, "food_delivery", false, nil); err != nil {
			return fmt.Errorf("bind queue %s to %s: %w", name, key, err)
		}
	}
	return nil
}

// Consume starts a non-blocking goroutine that processes messages from queue.
// Nacks (sends to DLQ) on handler error. Acks on success.
func (c *Consumer) Consume(ctx context.Context, queue string, handler HandlerFunc) error {
	if err := c.channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set QoS: %w", err)
	}

	msgs, err := c.channel.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				if err := handler(ctx, msg.Body); err != nil {
					log.Printf("[%s] handler error: %v — nacking to DLQ", queue, err)
					msg.Nack(false, false)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

func (c *Consumer) Close() {
	c.channel.Close()
}
