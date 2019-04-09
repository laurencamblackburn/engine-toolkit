package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/veritone/go-messaging-lib/kafka"
)

func TestEdgeEventData(t *testing.T) {
	// create random EdgeEventData message
	origMsg := RandomEdgeEventData()

	// encode for producing to Kafka
	msg, err := origMsg.ToKafka()
	assert.Nil(t, err)

	// simulate receiving an event message from Kafka
	kafkaMsg, ok := msg.Message().(*kafka.Message)
	assert.True(t, ok)
	ev := event{kafkaMsg}

	// verify message key
	key, err := GetMsgKey(&ev)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s-%s-%s", string(origMsg.Event), origMsg.Component, origMsg.JobID), key)

	// verify message type
	msgType, err := GetMsgType(&ev)
	assert.Nil(t, err)
	assert.Equal(t, EdgeEventDataType, msgType)

	// verify taskID
	taskID, err := GetTaskID(&ev)
	assert.Nil(t, err)
	assert.Equal(t, origMsg.TaskID, taskID)

	// decode back to EdgeEventData message
	newMsg, err := ToEdgeEventData(&ev)
	assert.Nil(t, err)

	// verify decode produced the original msg before encode
	assert.Equal(t, origMsg, newMsg)
}