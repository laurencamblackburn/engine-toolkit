package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
)

// Engine consumes messages and calls webhooks to
// fulfil the requests.
type Engine struct {
	producer Producer
	consumer Consumer
	testMode bool

	client   *http.Client
	logDebug func(args ...interface{})

	// Config holds the Engine configuration.
	Config Config
	BuildID string
	CurrentChunkID string
	CurrentJobID string
	CurrentTaskID string
	engineInstanceStartedDate time.Time
	lastReceivedDate time.Time
	processingDurationSecs int64
	chunkInPrefix string
}
type engineOutput struct {
	// SourceEngineID   string         `json:"sourceEngineId,omitempty"`
	// SourceEngineName string         `json:"sourceEngineName,omitempty"`
	// TaskPayload      payload        `json:"taskPayload,omitempty"`
	// TaskID           string         `json:"taskId"`
	// EntityID         string         `json:"entityId,omitempty"`
	// LibraryID        string         `json:"libraryId"`
	Series []seriesObject `json:"series"`
}

type seriesObject struct {
	Start     int    `json:"startTimeMs"`
	End       int    `json:"stopTimeMs"`
	EntityID  string `json:"entityId"`
	LibraryID string `json:"libraryId"`
	Object    object `json:"object"`
}

type object struct {
	Label        string   `json:"label"`
	Text         string   `json:"text"`
	ObjectType   string   `json:"type"`
	URI          string   `json:"uri"`
	EntityID     string   `json:"entityId,omitempty"`
	LibraryID    string   `json:"libraryId,omitempty"`
	Confidence   float64  `json:"confidence"`
	BoundingPoly []coords `json:"boundingPoly"`
}

type coords struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NewEngine makes a new Engine with the specified Consumer and Producer.
func NewEngine() *Engine {
	return &Engine{
		logDebug: func(args ...interface{}) {
			log.Println(args...)
		},
		Config: NewConfig(),
		client: http.DefaultClient,
		lastReceivedDate: time.Now(),
		engineInstanceStartedDate: time.Now(),
		chunkInPrefix:  "chunk_in_",
	}
}

// isTrainingTask gets whether the task is a training task or not.
func isTrainingTask() (bool, error) {
	payload, err := EnvPayload()
	if err == ErrNoPayload {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return payload.Mode == "library-train", nil
}

// Run runs the Engine.
// Context errors may be returned.
func (e *Engine) Run(ctx context.Context) error {
	isTraining, err := isTrainingTask()
	if err != nil {
		return errors.Wrap(err, "isTrainingTask")
	}
	if e.testMode {
		go e.runTestConsole(ctx)
		e.logDebug("running subprocess for testing...")
		return e.runSubprocessOnly(ctx)
	}
	if isTraining {
		e.logDebug("running subprocess for training...")
		return e.runSubprocessOnly(ctx)
	}
	e.logDebug("running inference...")
	return e.runInference(ctx)
}

// runSubprocessOnly starts the subprocess and doesn't do anything else.
// This is used for training tasks.
func (e *Engine) runSubprocessOnly(ctx context.Context) error {
	if len(e.Config.Subprocess.Arguments) < 1 {
		return errors.New("not enough arguments to run subprocess")
	}
	cmd := exec.CommandContext(ctx, e.Config.Subprocess.Arguments[0], e.Config.Subprocess.Arguments[1:]...)
	cmd.Stdout = e.Config.Stdout
	cmd.Stderr = e.Config.Stderr
	cmd.Stderr = e.Config.Stderr // TODO: deal with stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, e.Config.Subprocess.Arguments[0])
	}
	return nil
}

// runInference starts the subprocess and routes work to webhooks.
// This is used to process files.
func (e *Engine) runInference(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var cmd *exec.Cmd
	if len(e.Config.Subprocess.Arguments) > 0 {
		cmd = exec.CommandContext(ctx, e.Config.Subprocess.Arguments[0], e.Config.Subprocess.Arguments[1:]...)
		cmd.Stdout = e.Config.Stdout
		cmd.Stderr = e.Config.Stderr
		if err := cmd.Start(); err != nil {
			return errors.Wrap(err, e.Config.Subprocess.Arguments[0])
		}
		readyCtx, cancel := context.WithTimeout(ctx, e.Config.Subprocess.ReadyTimeout)
		defer cancel()
		e.logDebug("waiting for ready... will expire after", e.Config.Subprocess.ReadyTimeout)
		if err := e.ready(readyCtx); err != nil {
			return err
		}
	}
	e.logDebug("waiting for messages...")
	go func() {
		defer func() {
			e.logDebug("shutting down...")
			cancel()
		}()
		for {
			select {
			case msg, ok := <-e.consumer.Messages():
				if !ok {
					return
				}
				if err := e.processMessage(ctx, msg); err != nil {
					e.logDebug(fmt.Sprintf("processing error: %v", err))
				}
				e.consumer.MarkOffset(msg, "")
				resetCurrentTask(e)
			case <-time.After(e.Config.EndIfIdleDuration):
				e.logDebug(fmt.Sprintf("idle for %s", e.Config.EndIfIdleDuration))

				return
			case <-ctx.Done():
				return
			}
		}
	}()
	if cmd != nil {
		// wait for the command
		if err := cmd.Wait(); err != nil {
			if err := ctx.Err(); err != nil {
				// if the context has an error, we'll assume this command
				// errored because we terminated it (via context).
				return ctx.Err()
			}
			// otherwise, the subprocess has crashed
			return errors.Wrap(err, e.Config.Subprocess.Arguments[0])
		}
		return nil
	}
	<-ctx.Done()
	defer func() {
		e.logDebug("Send edge event message when engine instance gracefully shutdown")
		//Send edge event message when engine instance gracefully shutdown
		edgeMessage := EmptyEdgeEventData()
		edgeMessage.Event = EngineInstanceQuit
		edgeMessage.EventTimeUTC = getCurrentTimeEpochMs()
		edgeMessage.Component = e.Config.EngineID
		edgeMessage.EngineInfo.EngineID = e.Config.EngineID
		edgeMessage.EngineInfo.BuildID = e.BuildID
		edgeMessage.EngineInfo.InstanceID =  e.Config.EngineInstanceID
		e.logDebug(newJSONEncoder(e.producer))
		_, _, err := e.producer.SendMessage(&sarama.ProducerMessage{
			Topic: e.Config.EdgeEventTopic,
			Key:   sarama.ByteEncoder(e.Config.EngineID),
			Value: newJSONEncoder(edgeMessage),
		})
		if err != nil {
			errors.Wrapf(err, "SendMessage: %q %s", e.Config.EngineID, EngineInstanceQuit)
		}
	}()
	return nil
}

func (e *Engine) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var typeCheck struct {
		Type messageType
	}
	if err := json.Unmarshal(msg.Value, &typeCheck); err != nil {
		return errors.Wrap(err, "unmarshal message value JSON")
	}
	switch typeCheck.Type {
	case messageTypeMediaChunk:
		if err := e.processMessageMediaChunk(ctx, msg); err != nil {
			return errors.Wrap(err, "process media chunk")
		}
	default:
		e.logDebug(fmt.Sprintf("ignoring message of type %q: %+v", typeCheck.Type, msg))
	}
	return nil
}

// processMessageMediaChunk processes a single media chunk as described by the sarama.ConsumerMessage.
func (e *Engine) processMessageMediaChunk(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var mediaChunk mediaChunkMessage
	if err := json.Unmarshal(msg.Value, &mediaChunk); err != nil {
		return errors.Wrap(err, "unmarshal message value JSON")
	}
	inputMessage, er := json.Marshal(mediaChunk)
	if er != nil {
		return errors.Wrapf(er, "json: marshal engine with input message of taskID: %s", mediaChunk.TaskID)
	}
	e.logDebug("Engine process with input message: ", string(inputMessage))

	// Update current job and task
	updateCurrentTask(e, mediaChunk)
	//send edge event message when consumed a Kafka message from its engine topic
	edgeMessage := EmptyEdgeEventData()
	edgeMessage.Event = MediaChunkConsumed
	edgeMessage.EventTimeUTC = getCurrentTimeEpochMs()
	edgeMessage.Component = e.Config.EngineID
	edgeMessage.JobID = e.CurrentJobID
	edgeMessage.TaskID = e.CurrentTaskID
	edgeMessage.ChunkID = e.CurrentChunkID
	edgeMessage.EngineInfo.EngineID = e.Config.EngineID
	edgeMessage.EngineInfo.BuildID = e.BuildID
	edgeMessage.EngineInfo.InstanceID =  e.Config.EngineInstanceID

	_, _, errSend := e.producer.SendMessage(&sarama.ProducerMessage{
		Topic: e.Config.EdgeEventTopic,
		Key:   sarama.ByteEncoder(mediaChunk.ChunkUUID),
		Value: newJSONEncoder(edgeMessage),
	})
	if errSend != nil {
		errors.Wrapf(errSend, "SendMessage: %q %s", mediaChunk.ChunkUUID, MediaChunkConsumed)
	}
	finalUpdateMessage := chunkResult{
		Type:      messageTypeChunkResult,
		TaskID:    mediaChunk.TaskID,
		ChunkUUID: mediaChunk.ChunkUUID,
		Status:    chunkStatusSuccess, // optimistic
	}
	defer func() {
		// send the final (ChunkResult) message
		finalUpdateMessage.TimestampUTC = time.Now().Unix()
		_, _, err := e.producer.SendMessage(&sarama.ProducerMessage{
			Topic: e.Config.Kafka.ChunkTopic,
			Key:   sarama.ByteEncoder(msg.Key),
			Value: newJSONEncoder(finalUpdateMessage),
		})
		if err != nil {
			e.logDebug(fmt.Sprintf("WARN: failed to send final chunk update of taskID: %s with error: %s", mediaChunk.TaskID, err.Error()))
		}
		// update Processing Duration Secs
		updateProcessingDurationSecs(e)
		// send edge event message when producing a Kafka message to chunk_all topic
		edgeMessage := EmptyEdgeEventData()
		edgeMessage.Event = ChunkResultProduced
		edgeMessage.EventTimeUTC = getCurrentTimeEpochMs()
		edgeMessage.Component = e.Config.EngineID
		edgeMessage.JobID = e.CurrentJobID
		edgeMessage.TaskID = e.CurrentTaskID
		edgeMessage.ChunkID = e.CurrentChunkID
		edgeMessage.EngineInfo.EngineID = e.Config.EngineID
		edgeMessage.EngineInfo.BuildID = e.BuildID
		edgeMessage.EngineInfo.InstanceID =  e.Config.EngineInstanceID

		_, _, er := e.producer.SendMessage(&sarama.ProducerMessage{
			Topic: e.Config.EdgeEventTopic,
			Key:   sarama.ByteEncoder(mediaChunk.ChunkUUID),
			Value: newJSONEncoder(edgeMessage),
		})
		if er != nil {
			errors.Wrapf(err, "SendMessage: %q %s", mediaChunk.ChunkUUID, ChunkResultProduced)
		}
	}()

	ignoreChunk := false
	retry := newDoubleTimeBackoff(
		e.Config.Webhooks.Backoff.InitialBackoffDuration,
		e.Config.Webhooks.Backoff.MaxBackoffDuration,
		e.Config.Webhooks.Backoff.MaxRetries,
	)
	var content []byte
	var engineOutputContent engineOutput
	err := retry.Do(func() error {
		req, err := newRequestFromMediaChunk(e.client, e.Config.Webhooks.Process.URL, mediaChunk)
		if err != nil {
			return errors.Wrap(err, "new request")
		}
		req = req.WithContext(ctx)
		resp, err := e.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, resp.Body); err != nil {
			return errors.Wrap(err, "read body")
		}
		if resp.StatusCode == http.StatusNoContent {
			ignoreChunk = true
			return nil
		}
		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("%d: %s", resp.StatusCode, buf.String())
		}
		if buf.Len() == 0 {
			ignoreChunk = true
			return nil
		}
		if err := json.NewDecoder(&buf).Decode(&engineOutputContent); err != nil {
			return errors.Wrap(err, "decode response")
		}
		return nil
	})
	if err != nil {
		// send error message
		errMessage := fmt.Sprintf("Unable to process taskID %s: %v", mediaChunk.TaskID, err.Error())
		err = errors.Wrapf(err, "SendMessage: %q %s %s", e.Config.Kafka.ChunkTopic, messageTypeChunkProcessedStatus, chunkStatusSuccess)
		finalUpdateMessage.Status = chunkStatusError
		finalUpdateMessage.ErrorMsg = err.Error()
		finalUpdateMessage.FailureReason = FailureReasonInternalError
		finalUpdateMessage.FailureMsg = errMessage
		return err
	}
	if ignoreChunk {
		finalUpdateMessage.Status = chunkStatusIgnored
		return nil
	}
	content, err = json.Marshal(engineOutputContent)
	if err != nil {
		return errors.Wrapf(err, "json: marshal engine output content of taskID: %s", mediaChunk.TaskID)
	}
	// send output message
	outputMessage := mediaChunkMessage{
		Type:          messageTypeEngineOutput,
		TaskID:        mediaChunk.TaskID,
		JobID:         mediaChunk.JobID,
		ChunkUUID:     mediaChunk.ChunkUUID,
		StartOffsetMS: mediaChunk.StartOffsetMS,
		EndOffsetMS:   mediaChunk.EndOffsetMS,
		TimestampUTC:  time.Now().Unix(),
		Content:       string(content),
	}
	tmp, _ := json.Marshal(outputMessage)
	e.logDebug("outputMessage will be sent to kafka: ", string(tmp))
	finalUpdateMessage.EngineOutput = &outputMessage
	return nil
}

// ready returns a channel that is closed when the engine is
// ready.
// The channel may receive an error if something goes wrong while waiting
// for the engine to become ready.
func (e *Engine) ready(ctx context.Context) error {
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().Sub(start) >= e.Config.Webhooks.Ready.MaximumPollDuration {
			e.logDebug("ready: exceeded", e.Config.Webhooks.Ready.MaximumPollDuration)
			return errReadyTimeout
		}
		resp, err := http.Get(e.Config.Webhooks.Ready.URL)
		if err != nil {
			e.logDebug("ready: err:", err)
			time.Sleep(e.Config.Webhooks.Ready.PollDuration)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			e.logDebug("ready: status:", resp.Status)
			time.Sleep(e.Config.Webhooks.Ready.PollDuration)
			continue
		}
		e.logDebug("ready: yes")
		return nil
	}
}

func nolog(args ...interface{}) {}

// errReadyTimeout is sent down the Ready channel if the
// Webhooks.Ready.MaximumPollDuration is exceeded.
var errReadyTimeout = errors.New("ready: maximum duration exceeded")

// jsonEncoder encodes JSON.
type jsonEncoder struct {
	v    interface{}
	once sync.Once
	b    []byte
	err  error
}

func newJSONEncoder(v interface{}) sarama.Encoder {
	return &jsonEncoder{v: v}
}

func (j *jsonEncoder) encode() {
	j.once.Do(func() {
		j.b, j.err = json.Marshal(j.v)
	})
}

func (j *jsonEncoder) Encode() ([]byte, error) {
	j.encode()
	return j.b, j.err
}

func (j *jsonEncoder) Length() int {
	j.encode()
	return len(j.b)
}
func setBuidEngine(e * Engine)  {
	e.BuildID = strings.Replace(e.Config.Kafka.ChunkTopic, e.chunkInPrefix, "", -1)
}
func resetCurrentTask(e * Engine) {
	e.CurrentJobID = ""
	e.CurrentTaskID = ""
	e.CurrentChunkID = ""
}
func updateCurrentTask(e * Engine, job mediaChunkMessage) {
	e.CurrentJobID = job.JobID
	e.CurrentTaskID = job.TaskID
	e.CurrentChunkID = job.ChunkUUID
}
func updateProcessingDurationSecs(e * Engine) {
	e.processingDurationSecs += int64(time.Now().Sub(e.lastReceivedDate).Seconds())
}
func timeEngineInstancePeriodic(e *Engine) {
	// Convert tp duration
	timeIntervalInDuration, _ := time.ParseDuration(e.Config.TimeToSendPeriodicMessageInDuration)

	timeTicker := time.NewTicker(timeIntervalInDuration)
	go func() {
		for range timeTicker.C {
			// Send edge event message  total time (rounded to nearest second) engine instance has been and total time processing
			edgeMessage := EmptyEdgeEventData()
			edgeMessage.Event = EngineInstancePeriodic
			edgeMessage.EventTimeUTC = getCurrentTimeEpochMs()
			edgeMessage.Component = e.Config.EngineID
			edgeMessage.JobID = e.CurrentJobID
			edgeMessage.TaskID = e.CurrentTaskID
			edgeMessage.EngineInfo.EngineID = e.Config.EngineID
			edgeMessage.EngineInfo.BuildID = e.BuildID
			edgeMessage.EngineInfo.InstanceID =  e.Config.EngineInstanceID
			edgeMessage.EngineInfo.ProcessingDurationSecs = e.processingDurationSecs
			edgeMessage.EngineInfo.UpDurationSecs = int64(time.Now().Sub(e.engineInstanceStartedDate).Seconds())

			_, _, err := e.producer.SendMessage(&sarama.ProducerMessage{
				Topic: e.Config.EdgeEventTopic,
				Key:   sarama.ByteEncoder(e.Config.EngineID),
				Value: newJSONEncoder(edgeMessage),
			})
			if err != nil {
				errors.Wrapf(err, "SendMessage: %q %s", e.Config.EngineID, EngineInstancePeriodic)
			}
		}
	}()
}