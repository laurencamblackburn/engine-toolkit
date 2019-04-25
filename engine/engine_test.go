package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/matryer/is"
)

// TestProcessingChunk tests the entire end to end flow of processing
// a chunk.
func TestProcessingChunk(t *testing.T) {
	is := is.New(t)

	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{} // no subprocess
	engine.Config.Kafka.ChunkTopic = "chunk-topic"
	engine.Config.EngineID = "engine-1"
	engine.BuildID = "build-1"
	engine.logDebug = func(args ...interface{}) {}
	inputPipe := newPipe()
	defer inputPipe.Close()
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.consumer = inputPipe
	engine.producer = outputPipe
	readySrv := newOKServer()
	defer readySrv.Close()
	engine.Config.Webhooks.Ready.URL = readySrv.URL
	processSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engineOutput := engineOutput{
			Series: []seriesObject{
				{
					Object: object{
						Label: "something",
					},
				},
			},
		}
		err := json.NewEncoder(w).Encode(engineOutput)
		is.NoErr(err)
	}))
	defer processSrv.Close()
	engine.Config.Webhooks.Process.URL = processSrv.URL

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := engine.Run(ctx)
		is.NoErr(err)
	}()

	inputMessage := mediaChunkMessage{
		TimestampUTC:  time.Now().Unix(),
		ChunkUUID:     "123",
		Type:          messageTypeMediaChunk,
		StartOffsetMS: 1000,
		EndOffsetMS:   2000,
		JobID:         "job1",
		TDOID:         "tdo1",
		TaskID:        "task1",
	}
	var outputMsg *sarama.ConsumerMessage
	var chunkResult chunkResult
	_, _, err := inputPipe.SendMessage(&sarama.ProducerMessage{
		Offset: 1,
		Key:    sarama.StringEncoder(inputMessage.TaskID),
		Value:  newJSONEncoder(inputMessage),
	})
	is.NoErr(err)

	// read the media_chunk_consumed success message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}
	is.Equal(outputMsg.Topic, engine.Config.EdgeEventTopic)  // edge event topic
	is.Equal(string(outputMsg.Key), inputMessage.ChunkUUID)  // chunk input

	// check for the final chunk output message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}
	is.Equal(string(outputMsg.Key), inputMessage.TaskID)      // output message key must be TaskID
	is.Equal(outputMsg.Topic, engine.Config.Kafka.ChunkTopic) // chunk topic
	err = json.Unmarshal(outputMsg.Value, &chunkResult)
	is.NoErr(err)
	is.Equal(chunkResult.ErrorMsg, "")
	is.Equal(chunkResult.Type, messageTypeChunkResult)
	is.Equal(chunkResult.TaskID, inputMessage.TaskID)
	is.Equal(chunkResult.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(chunkResult.Status, chunkStatusSuccess)

	is.Equal(chunkResult.EngineOutput.Type, messageTypeEngineOutput)
	is.Equal(chunkResult.EngineOutput.TaskID, inputMessage.TaskID)
	is.Equal(chunkResult.EngineOutput.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(chunkResult.EngineOutput.StartOffsetMS, 1000)
	is.Equal(chunkResult.EngineOutput.EndOffsetMS, 2000)

	// read the chunk success message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}
	is.Equal(string(outputMsg.Key), inputMessage.ChunkUUID) // output message key must be TaskID
	is.Equal(outputMsg.Topic, engine.Config.EdgeEventTopic) // edge event topic

	var output engineOutput
	err = json.Unmarshal([]byte(chunkResult.EngineOutput.Content), &output)
	is.NoErr(err)
	is.Equal(len(output.Series), 1)
	is.Equal(output.Series[0].Object.Label, "something")
	is.Equal(inputPipe.Offset, int64(1)) // Offset
}

func TestReadiness(t *testing.T) {
	is := is.New(t)
	var lock sync.Mutex
	var ready bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			ready = true
		}
	}))
	defer srv.Close()
	engine := NewEngine()
	engine.logDebug = func(args ...interface{}) {}
	engine.Config.Webhooks.Ready.URL = srv.URL
	engine.Config.Webhooks.Ready.PollDuration = 10 * time.Millisecond
	engine.Config.Webhooks.Ready.MaximumPollDuration = 1 * time.Second
	err := engine.ready(context.Background())
	is.NoErr(err)
}

func TestReadinessMaximumPollDuration(t *testing.T) {
	is := is.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	engine := NewEngine()
	engine.logDebug = func(args ...interface{}) {}
	engine.Config.Webhooks.Ready.URL = srv.URL
	engine.Config.Webhooks.Ready.PollDuration = 10 * time.Millisecond
	engine.Config.Webhooks.Ready.MaximumPollDuration = 100 * time.Millisecond
	err := engine.ready(context.Background())
	is.Equal(err, errReadyTimeout)
}

func TestReadinessContextCancelled(t *testing.T) {
	is := is.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	engine := NewEngine()
	engine.logDebug = func(args ...interface{}) {}
	engine.Config.Webhooks.Ready.URL = srv.URL
	engine.Config.Webhooks.Ready.PollDuration = 10 * time.Millisecond
	engine.Config.Webhooks.Ready.MaximumPollDuration = 1 * time.Second
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	err := engine.ready(ctx)
	is.Equal(err, context.DeadlineExceeded)

}

func TestSubprocess(t *testing.T) {
	is := is.New(t)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{"echo", "-n", "something"}
	inputPipe := newPipe()
	defer inputPipe.Close()
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.consumer = inputPipe
	engine.producer = outputPipe
	readySrv := newOKServer()
	defer readySrv.Close()
	engine.Config.Webhooks.Ready.URL = readySrv.URL

	var buf bytes.Buffer
	engine.Config.Stdout = &buf

	// engine will run until the subprocess exits
	err := engine.Run(ctx)
	is.NoErr(err)
	is.Equal(buf.String(), `something`)
}

func TestEndIfIdleDuration(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{} // no subprocess
	engine.Config.EndIfIdleDuration = 100 * time.Millisecond
	inputPipe := newPipe()
	defer inputPipe.Close()
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.consumer = inputPipe
	engine.producer = outputPipe
	err := engine.Run(ctx)
	is.NoErr(err)
}

func newOKServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
}

func TestIsTrainingTask(t *testing.T) {
	is := is.New(t)

	os.Setenv("PAYLOAD_FILE", "")
	isTraining, err := isTrainingTask()
	is.NoErr(err)
	is.Equal(isTraining, false) // not training

	os.Setenv("PAYLOAD_FILE", "testdata/training-task-payload.json")
	isTraining, err = isTrainingTask()
	is.NoErr(err)
	is.Equal(isTraining, true) // training task

	os.Setenv("PAYLOAD_FILE", "testdata/processing-task-payload.json")
	isTraining, err = isTrainingTask()
	is.NoErr(err)
	is.Equal(isTraining, false) // not training task
}

func TestIgnoredChunks(t *testing.T) {
	is := is.New(t)

	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{} // no subprocess
	engine.Config.Kafka.ChunkTopic = "chunk-topic"
	engine.logDebug = func(args ...interface{}) {}
	inputPipe := newPipe()
	defer inputPipe.Close()
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.consumer = inputPipe
	engine.producer = outputPipe
	readySrv := newOKServer()
	defer readySrv.Close()
	engine.Config.Webhooks.Ready.URL = readySrv.URL
	processSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer processSrv.Close()
	engine.Config.Webhooks.Process.URL = processSrv.URL

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := engine.Run(ctx)
		is.NoErr(err)
	}()

	inputMessage := mediaChunkMessage{
		TimestampUTC:  time.Now().Unix(),
		ChunkUUID:     "123",
		Type:          messageTypeMediaChunk,
		StartOffsetMS: 1000,
		EndOffsetMS:   2000,
		JobID:         "job1",
		TDOID:         "tdo1",
		TaskID:        "task1",
	}
	_, _, err := inputPipe.SendMessage(&sarama.ProducerMessage{
		Offset: 1,
		Key:    sarama.StringEncoder(inputMessage.TaskID),
		Value:  newJSONEncoder(inputMessage),
	})
	is.NoErr(err)

	var outputMsg *sarama.ConsumerMessage
	var chunkProcessedStatus chunkProcessedStatus

	// read the media_chunk_consumed success message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}

	is.Equal(outputMsg.Topic, engine.Config.EdgeEventTopic)  // edge event topic
	is.Equal(string(outputMsg.Key), inputMessage.ChunkUUID)  // chunk input

	// read the chunk ignore message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}
	is.Equal(string(outputMsg.Key), inputMessage.TaskID)      // output message key must be TaskID
	is.Equal(outputMsg.Topic, engine.Config.Kafka.ChunkTopic) // chunk topic
	err = json.Unmarshal(outputMsg.Value, &chunkProcessedStatus)
	is.NoErr(err)
	is.Equal(chunkProcessedStatus.ErrorMsg, "")
	is.Equal(chunkProcessedStatus.Type, messageTypeChunkResult)
	is.Equal(chunkProcessedStatus.TaskID, inputMessage.TaskID)
	is.Equal(chunkProcessedStatus.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(chunkProcessedStatus.Status, chunkStatusIgnored)

	is.Equal(inputPipe.Offset, int64(1)) // Offset
}

func TestSubprocessCrash(t *testing.T) {
	is := is.New(t)

	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{"./testdata/subprocesses/crash.sh"}
	engine.Config.Kafka.ChunkTopic = "chunk-topic"
	engine.logDebug = func(args ...interface{}) {
		log.Println(args...)
	}
	readySrv := newOKServer()
	defer readySrv.Close()
	engine.Config.Webhooks.Ready.URL = readySrv.URL
	inputPipe := newPipe()
	defer inputPipe.Close()
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.consumer = inputPipe
	engine.producer = outputPipe

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := engine.Run(ctx)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "exit status 123"))

}

type engineOutputMessage struct {
	Type          messageType `json:"type"`
	TimestampUTC  int64       `json:"timestampUTC"`
	OuputType     string      `json:"ouputType"`
	MIMEType      string      `json:"mimeType"`
	TaskID        string      `json:"taskId"`
	TDOID         string      `json:"tdoId"`
	JobID         string      `json:"jobId"`
	StartOffsetMS int         `json:"startOffsetMs"`
	EndOffsetMS   int         `json:"endOffsetMs"`
	Content       string      `json:"content,omitempty"`
	Rev           int64       `json:"rev"`
	TaskPayload   payload     `json:"taskPayload"`
	ChunkUUID     string      `json:"chunkUUID"`
}