package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/satori/go.uuid"
	"github.com/veritone/go-messaging-lib"
	"github.com/veritone/go-messaging-lib/kafka"
)

// EdgeMessageType - enum for edge message types
type EdgeMessageType string

// ValidMessageTypes - array contains all valid message types for Edge messages
// Note: new message types should be added here, ordering doesn't matter, but recommended in alphabetical order for readability
var ValidMessageTypes = [...]EdgeMessageType{
	EdgeEventDataType,
}

// Message is interface that all Edge messages must implement
type Message interface {
	Key() string
	ToKafka() (messaging.Messager, error)
}

// newMessage - creates new message suitable for producing to Kafka
func newMessage(key string, value interface{}) (messaging.Messager, error) {
	var valBytes []byte
	var err error

	switch v := value.(type) {
	case []byte:
		valBytes = v
	default:
		valBytes, err = encode(value)
		if err != nil {
			return nil, fmt.Errorf("cannot convert message value to bytes: %v", err)
		}
	}

	return kafka.NewMessage(key, valBytes)
}

// encode - encode any payload to []byte suitable for producing to Kafka
func encode(payload interface{}) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// decode - decode []byte from Kafka into concrete structure
func decode(from []byte, to interface{}) error {
	err := json.Unmarshal(from, &to)
	if err != nil {
		return fmt.Errorf("error unmarshalling: %v", err)
	}
	return nil
}

// msgTypeOnly - temp struct for getting/testing message type
type msgTypeOnly struct {
	Type EdgeMessageType
}

// GetMsgType - given a Kafka message, determine what type of Edge message it is
func GetMsgType(e messaging.Event) (EdgeMessageType, error) {
	m := msgTypeOnly{}
	err := json.Unmarshal(e.Payload(), &m)
	if err != nil {
		return "", fmt.Errorf("error getting type for message: err = %v, msg = %v", err, e.Raw())
	}
	return m.Type, nil
}

func uuidv4() string {
	id, err := uuid.NewV4()
	if err != nil {
		return "12345678-90ab-cdef-1234-567809ab"
	}
	return id.String()
}

// Event implements Event interface in go-messaging-lib (for testing purpose of simulating an Event from Kafka)
type event struct {
	*kafka.Message
}

func (e *event) Payload() []byte {
	return e.Value
}

func (e *event) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"key":       e.Key,
		"offset":    e.Offset,
		"partition": e.Partition,
		"time":      e.Time,
		"topic":     e.Topic,
	}
}

func (e *event) Raw() interface{} {
	return e.Message
}

// GetMsgKey gets message key from Kafka event message
func GetMsgKey(e messaging.Event) (string, error) {
	metadata := e.Metadata()
	if metadata == nil {
		return "", errors.New("metadata is nil")
	}
	keyBytes, ok := metadata["key"]
	if !ok {
		return "", errors.New("metadata does not have key field")
	}
	// need to convert to []uint8 first before convert to string
	// otherwise string will be empty
	keyBytesAsUint8, ok := keyBytes.([]uint8)
	if !ok {
		return "", errors.New("cannot convert key field to string")
	}
	return string(keyBytesAsUint8), nil
}

// GetTaskID gets taskID field from Kafka event message if available
func GetTaskID(e messaging.Event) (string, error) {
	msgType, err := GetMsgType(e)
	if err != nil {
		return "", err
	}
	switch msgType {
	case EdgeEventDataType:
		m, err := ToEdgeEventData(e)
		if err != nil {
			return "", err
		}
		return m.TaskID, nil
	}
	return "", fmt.Errorf("Unknown message type: %v", msgType)
	}
// get current time UTC in milliseconds since epoch
func getCurrentTimeEpochMs() int64 {
	return int64(time.Now().UTC().UnixNano() / 1e6)
}
