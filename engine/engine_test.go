package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	_, _, err := inputPipe.SendMessage(&sarama.ProducerMessage{
		Offset: 1,
		Key:    sarama.StringEncoder(inputMessage.TaskID),
		Value:  newJSONEncoder(inputMessage),
	})
	is.NoErr(err)

	var outputMsg *sarama.ConsumerMessage
	var chunkProcessedStatus chunkProcessedStatus

	// read the chunk processing message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
	}
	is.Equal(string(outputMsg.Key), inputMessage.TaskID)      // output message key must be TaskID
	is.Equal(outputMsg.Topic, engine.Config.Kafka.ChunkTopic) // chunk topic
	err = json.Unmarshal(outputMsg.Value, &chunkProcessedStatus)
	is.NoErr(err)
	is.Equal(chunkProcessedStatus.Type, messageTypeChunkProcessedStatus)
	is.Equal(chunkProcessedStatus.TaskID, inputMessage.TaskID)
	is.Equal(chunkProcessedStatus.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(chunkProcessedStatus.Status, chunkStatusProcessing)
	is.Equal(chunkProcessedStatus.ErrorMsg, "")

	// check for the final chunk output message
	select {
	case outputMsg = <-outputPipe.Messages():
	case <-time.After(1 * time.Second):
		is.Fail() // timed out
		return
	}
	is.Equal(string(outputMsg.Key), inputMessage.TaskID)      // output message key must be TaskID
	is.Equal(outputMsg.Topic, engine.Config.Kafka.ChunkTopic) // chunk topic
	var engineOutputMessage engineOutputMessage
	err = json.Unmarshal(outputMsg.Value, &engineOutputMessage)
	is.Equal(engineOutputMessage.Type, messageTypeEngineOutput)
	is.Equal(engineOutputMessage.TaskID, inputMessage.TaskID)
	is.Equal(engineOutputMessage.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(engineOutputMessage.StartOffsetMS, 1000)
	is.Equal(engineOutputMessage.EndOffsetMS, 2000)

	// read the chunk success message
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
	is.Equal(chunkProcessedStatus.Type, messageTypeChunkProcessedStatus)
	is.Equal(chunkProcessedStatus.TaskID, inputMessage.TaskID)
	is.Equal(chunkProcessedStatus.ChunkUUID, inputMessage.ChunkUUID)
	is.Equal(chunkProcessedStatus.Status, chunkStatusSuccess)

	var output engineOutput
	err = json.Unmarshal([]byte(engineOutputMessage.Content), &output)
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

func TestTaskProcessingInterval(t *testing.T) {
	t.Skip("skipping because we've disabled 'processing' updates")
	is := is.New(t)

	engine := NewEngine()
	engine.Config.Subprocess.Arguments = []string{} // no subprocess // no subprocess
	outputPipe := newPipe()
	defer outputPipe.Close()
	engine.producer = outputPipe

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	messageKey := []byte("key")
	taskID := "taskID"
	chunkUUID := "chunkUUID"

	runFor := 1100 * time.Millisecond
	engine.Config.Tasks.ProcessingUpdateInterval = 100 * time.Millisecond
	finished := engine.periodicallySendProgressMessage(ctx, messageKey, taskID, chunkUUID)

	go func() {
		time.Sleep(runFor)
		cancel()
	}()

	var messages []*sarama.ConsumerMessage
	go func() {
		for msg := range outputPipe.Messages() {
			messages = append(messages, msg)
		}
	}()

	select {
	case <-time.After(2 * time.Second):
		is.Fail() // didn't finish in time
	case <-finished:
	}

	is.Equal(len(messages), 10)

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
