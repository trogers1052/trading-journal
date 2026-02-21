package kafka

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/IBM/sarama"
	"github.com/trogers1052/trading-journal/internal/models"
)

// TradeHandler is called when a trade event is received
type TradeHandler func(ctx context.Context, event *models.TradeEvent) error

// Consumer wraps Sarama consumer group for Kafka consumption
type Consumer struct {
	client       sarama.ConsumerGroup
	ordersTopic  string
	tradeHandler TradeHandler
	ready        chan bool
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(brokers []string, groupID, ordersTopic string) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest // Start from beginning to catch all trades
	config.Version = sarama.V2_8_0_0

	client, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:      client,
		ordersTopic: ordersTopic,
		ready:       make(chan bool),
	}, nil
}

// SetTradeHandler sets the handler for trade events
func (c *Consumer) SetTradeHandler(handler TradeHandler) {
	c.tradeHandler = handler
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	topics := []string{c.ordersTopic}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			handler := &consumerGroupHandler{
				consumer: c,
				ready:    c.ready,
			}

			if err := c.client.Consume(ctx, topics, handler); err != nil {
				log.Printf("Error from consumer: %v", err)
			}

			if ctx.Err() != nil {
				return
			}

			c.ready = make(chan bool)
		}
	}()

	<-c.ready
	log.Println("Kafka consumer started and ready")
	return nil
}

// Close stops the consumer gracefully
func (c *Consumer) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return c.client.Close()
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer *Consumer
	ready    chan bool
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			ctx := session.Context()

			if h.consumer.tradeHandler != nil {
				var event models.TradeEvent
				if err := json.Unmarshal(message.Value, &event); err != nil {
					log.Printf("Failed to unmarshal trade event: %v", err)
					session.MarkMessage(message, "")
					continue
				}

				// Only process TRADE_DETECTED events
				if event.EventType == "TRADE_DETECTED" {
					if err := h.consumer.tradeHandler(ctx, &event); err != nil {
						log.Printf("Failed to handle trade event: %v", err)
						continue
					}
				}
			}

			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}
