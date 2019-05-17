package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/matryer/is"
	"math/rand"
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
// RandomEdgeEventData - creates an EdgeEventData message with random values
func RandomEdgeEventData() EdgeEventData {
	rand.Seed(time.Now().UnixNano())
	return EdgeEventData{
		Type:         EdgeEventDataType,
		TimestampUTC: getCurrentTimeEpochMs(),
		Event:        randomEvent(),
		EventTimeUTC: getCurrentTimeEpochMs(),
		Component:    uuidv4(),
		JobID:        uuidv4(),
		TaskID:       uuidv4(),
		ChunkID:      uuidv4(),
		EngineInfo: &EngineInfo{
			EngineID:               uuidv4(),
			BuildID:                uuidv4(),
			InstanceID:             uuidv4(),
			UpDurationSecs:         rand.Int63n(1e6),
			ProcessingDurationSecs: rand.Int63n(1e6),
			InstanceLimit:          rand.Int63n(1e6),
			NoResourceErr:          "CPU Error",
		},
	}
}

func randomEvent() EdgeEvent {
	switch rand.Intn(38) {
	case 0:
		return MediaChunkConsumed
	case 1:
		return ChunkResultProduced
	case 2:
		return EngineInstanceReady
	case 3:
		return EngineInstanceQuit
	default:
		return EngineInstancePeriodic
	}
}

func uuidv4() string {
	return "12345678-90ab-cdef-1234-567809ab"
}