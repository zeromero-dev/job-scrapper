package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaMessageFlow(t *testing.T) {
	const topic = "job_notifications"
	const testMessage = "test-kafka-message"

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	err := writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte("test"),
		Value: []byte(testMessage),
	})
	if err != nil {
		t.Fatalf("failed to write test message to Kafka: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:9092"},
		Topic:     topic,
		GroupID:   "test-group",
		Partition: 0,
		MinBytes:  1e3,
		MaxBytes:  1e6,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("failed to read message from Kafka: %v", err)
	}

	if !strings.Contains(string(msg.Value), testMessage) {
		t.Errorf("expected message to contain %q, got %q", testMessage, msg.Value)
	}
}
