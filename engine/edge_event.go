package main

import (
	"math/rand"
	"time"
)

// EdgeEvent an event of interest that occur on Edge
type EdgeEvent string

// EdgeMessageType - enum for edge message types
type EdgeMessageType string

const (
	// EdgeEventDataType - string ID of EdgeEvent message type
	EdgeEventDataType EdgeMessageType = "edge_event_data"

	// Notes:
	//   - "Produced" means when message is put into a Kafka topic
	//   - "Consumed" means when message is taken from Kafka topic
	// Chunk task lifecycle events

	// MediaChunkConsumed event when consumed a Kafka message from its engine topic
	MediaChunkConsumed EdgeEvent = "media_chunk_consumed"
	// ChunkResultProduced (or ChunkProcessedStatusProduced or EngineOutputProduced) event when producing a Kafka message to chunk_all topic
	ChunkResultProduced EdgeEvent = "chunk_result_produced"
	// EngineInstanceReady event when engine instance is ready to process work
	EngineInstanceReady EdgeEvent = "engine_instance_ready"
	// EngineInstanceQuit event when engine instance gracefully shutdown
	EngineInstanceQuit EdgeEvent = "engine_instance_quit"
	// EngineInstancePeriodic event reporting total time (rounded to nearest second) engine instance has been and total time processing
	EngineInstancePeriodic EdgeEvent = "engine_instance_up_periodic"
)

// EdgeEventData is produced by Edge services and engines whenever one of above events occur
type EdgeEventData struct {
	Type         EdgeMessageType             `json:"type,omitempty"`         // EdgeEventDataType (required)
	TimestampUTC int64                       `json:"timestampUTC,omitempty"` // TimestampUTC is time this message created in epoch millisecs (optional)
	Event        EdgeEvent                   `json:"event,omitempty"`        // Event is one of Edge events listed above (required)
	EventTimeUTC int64                       `json:"eventTimeUTC,omitempty"` // EventTimeUTC is time that event occurred in epoch millisecs (required)
	Component    string                      `json:"component,omitempty"`    // Component is service name or EngineID that generated the event (required)
	JobID        string                      `json:"jobId,omitempty"`        // JobID this event is associated with (required, except for EngineInstance* events)
	TaskID       string                      `json:"taskId,omitempty"`       // TaskID this event is associated with (required, except for EngineInstance* events)
	ChunkID      string                      `json:"chunkId,omitempty"`      // ChunkID this event is associated with (required, except for GQLCall, ChunkEOF, GenericEvent, RunTask, and EngineInstance events)
	EngineInfo   *EngineInfo                 `json:"engineInfo,omitempty"`   // EngineInfo required only for EngineInstance events
	TopicInfo    map[string]map[string]int64 `json:"topicInfo,omitempty"`    // TopicInfo required only for TopicPeriodic events, lag for each consumer group in each topic
}

// EngineInfo contains contextual data for EngineInstance* events
type EngineInfo struct {
	EngineID               string `json:"engineId,omitempty"`               // EngineID of instance (required)
	BuildID                string `json:"buildId,omitempty"`                // BuildID of instance (required)
	InstanceID             string `json:"instanceId,omitempty"`             // InstanceID is unique ID of instance (either AWS Task ID or random UUID) (required)
	UpDurationSecs         int64  `json:"upDurationSecs,omitempty"`         // UpDurationSecs Required only for EngineInstancePeriodic event, contains engine up time in seconds
	ProcessingDurationSecs int64  `json:"processingDurationSecs,omitempty"` // ProcessingDurationSecs required only for EngineInstancePeriodic event, contains engine processing time in seconds
	InstanceLimit          int64  `json:"instanceLimit,omitempty"`          // InstanceLimit required only for EngineInstanceLimitReached event, contains the concurrent engine instance limit
	NoResourceErr          string `json:"noResourceErr,omitempty"`          // NoResourceErr required only for EngineInstanceNoResource event, contains details about the error
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

func getCurrentTimeEpochMs() int64 {
	return int64(time.Now().UTC().UnixNano() / 1e6)
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