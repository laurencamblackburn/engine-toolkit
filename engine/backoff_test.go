package main

import (
	"errors"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	okAfter := 5
	calls := 0

	b := newDoubleTimeBackoff(100*time.Millisecond, 1*time.Second, 10)
	err, reason := b.Do(func() (error, TaskFailureReason) {
		calls++
		if calls > okAfter {
			return nil, ""
		}
		return errors.New("not ok"), FailureReasonUnknown
	})
	if err != nil {
		t.Fatalf("Error should be nil but is: %v Failure Reason: %s", err, reason)
	}
	if calls > 10 {
		t.Fatalf("Calls should be < 10 but is: %v", calls)
	}
}

func TestBackoffMaxCalls(t *testing.T) {
	okAfter := 5
	calls := 0

	b := newDoubleTimeBackoff(100*time.Millisecond, 1*time.Second, 2)
	err, reason := b.Do(func() (error, TaskFailureReason) {
		calls++
		if calls > okAfter {
			return nil, ""
		}
		return errors.New("not ok"), FailureReasonUnknown
	})
	if err == nil {
		t.Fatalf("Error should be not nil but is: %v Failure Reason: %s", err, reason)
	}
	if calls != 2 {
		t.Fatalf("Calls should be 2 but is: %v", calls)
	}
}
