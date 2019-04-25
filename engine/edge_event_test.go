package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/matryer/is"
	"os"
	"sync"
	"testing"
	"time"
)

func TestEdgeEventData(t *testing.T) {
	is := is.New(t)
	// create random EdgeEventData message
	origMsg := RandomEdgeEventData()
	engineID := "engine-1"
	consumerGroup := "test-group-2"

	if os.Getenv("KAFKATEST") == "" {
		t.Skip("skipping unless KAFKATEST=true")
	}
	topic := fmt.Sprintf("%v", time.Now().Unix())
	consumer, cleanup, err := newKafkaConsumer([]string{"localhost:9092"}, consumerGroup, topic)
	is.NoErr(err)

	defer cleanup()
	defer consumer.Close()
	var wg sync.WaitGroup
	time.Sleep(1 * time.Second)
	producer, err := newKafkaProducer([]string{"localhost:9092"})
	is.NoErr(err)
	defer producer.Close()
	wg.Add(1)
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(engineID),
		Value: newJSONEncoder(origMsg),
	})
	is.NoErr(err)
	wg.Wait()
}