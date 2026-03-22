package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	channel *amqp.Channel
}

func NewPublisher(conn *Connection) (*Publisher, error) {
	ch, err := conn.NewChannel()
	if err != nil {
		return nil, fmt.Errorf("publisher: open channel: %w", err)
	}
	return &Publisher{channel: ch}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	return p.channel.PublishWithContext(ctx,
		"food_delivery",
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (p *Publisher) Close() {
	p.channel.Close()
}
