package main

import (
	"fmt"
	"github.com/veritone/go-messaging-lib"
	"math/rand"
	"time"
)

type EdgeEvent string

const (
	// EdgeEventDataType - string ID of EdgeEvent message type
	EdgeEventDataType EdgeMessageType = "edge_event_data"

	// Notes:
	//   - "Produced" means when message is put into a Kafka topic
	//   - "Consumed" means when message is taken from Kafka topic

	// Chunk task lifecycle events
	MediaChunkConsumed     EdgeEvent = "media_chunk_consumed"
	ChunkResultProduced    EdgeEvent = "chunk_result_produced"
	EngineInstanceReady    EdgeEvent = "engine_instance_ready"       // when engine instance is ready to process work
	EngineInstanceQuit     EdgeEvent = "engine_instance_quit"        // when engine instance dies/shut down
	EngineInstancePeriodic EdgeEvent = "engine_instance_up_periodic" // sends periodically by engines

	// Prefix chunk in topic
	chunkInPrefix = "chunk_in_"
)

// EdgeEventData is produced by Edge services and engines whenever one of above events occur
type EdgeEventData struct {
	Type         EdgeMessageType `json:"type,omitempty"`         // EdgeEventDataType (required)
	TimestampUTC int64           `json:"timestampUTC,omitempty"` // Time this message created in epoch millisecs (optional)
	Event        EdgeEvent       `json:"event,omitempty"`        // One of Edge events listed above (required)
	EventTimeUTC int64           `json:"eventTimeUTC,omitempty"` // Time that event occurred in epoch millisecs (required)
	Component    string          `json:"component,omitempty"`    // Service name or EngineID that generated the event (required)
	JobID        string          `json:"jobId,omitempty"`        // JobID this event is associated with (required, except for EngineInstance* events)
	TaskID       string          `json:"taskId,omitempty"`       // TaskID this event is associated with (required, except for EngineInstance* events)
	ChunkID      string          `json:"chunkId,omitempty"`      // ChunkID this event is associated with (required, except for GQLCall, ChunkEOF, GenericEvent, RunTask, and EngineInstance events)
	EngineInfo   *EngineInfo     `json:"engineInfo,omitempty"`   // Required only for EngineInstance events
	TopicInfo    map[string]map[string]int64 `json:"topicInfo,omitempty"`    // Required only for TopicPeriodic events, lag for each consumer group in each topic
}

// EngineInfo contains contextual data for EngineInstance* events
type EngineInfo struct {
	EngineID               string 			`json:"engineId,omitempty"`               // EngineID of instance (required)
	BuildID                string           `json:"buildId,omitempty"`                // BuildID of instance (required)
	InstanceID             string           `json:"instanceId,omitempty"`             // Unique ID of instance (either AWS Task ID or random UUID) (required)
	UpDurationSecs         int64            `json:"upDurationSecs,omitempty"`         // Required only for EngineInstancePeriodic event, contains engine up time in seconds
	ProcessingDurationSecs int64            `json:"processingDurationSecs,omitempty"` // Required only for EngineInstancePeriodic event, contains engine processing time in seconds
	InstanceLimit          int64            `json:"instanceLimit,omitempty"`          // Required only for EngineInstanceLimitReached event, contains the concurrent engine instance limit
	NoResourceErr          string           `json:"noResourceErr,omitempty"`          // Required only for EngineInstanceNoResource event, contains details about the error
}

// EmptyEdgeEventData - creates an empty EdgeEventData message
func EmptyEdgeEventData() EdgeEventData {
	return EdgeEventData{
		Type:         EdgeEventDataType,
		TimestampUTC: getCurrentTimeEpochMs(),
		EngineInfo: &EngineInfo{

		},
	}
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
		TopicInfo: map[string]map[string]int64{
			"topic1": map[string]int64{
				"group1": rand.Int63n(1000),
				"group2": rand.Int63n(1000),
			},
			"topic2": map[string]int64{
				"group1":  rand.Int63n(1000),
				"group22": rand.Int63n(1000),
			},
		},
	}
}

// Key - returns the message key
func (m EdgeEventData) Key() string {
	return fmt.Sprintf("%s-%s-%s", string(m.Event), m.Component, m.JobID)
}

// ToKafka - encode from EdgeEventData message to Kafka message
func (m EdgeEventData) ToKafka() (messaging.Messager, error) {
	return newMessage(m.Key(), m)
}

// ToEdgeEventData - decode from Kafka message to EdgeEventData message
func ToEdgeEventData(e messaging.Event) (EdgeEventData, error) {
	msg := EdgeEventData{}
	err := decode(e.Payload(), &msg)
	if err != nil {
		return msg, fmt.Errorf("error decoding to EdgeEventData: %v", err)
	}
	return msg, nil
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
