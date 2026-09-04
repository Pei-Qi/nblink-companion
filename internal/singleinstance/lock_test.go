package singleinstance

import (
	"errors"
	"testing"
	"time"
)

func TestAcquireActivatesExistingInstance(t *testing.T) {
	first, err := acquireAt("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := acquireAt(first.listener.Addr().String())
	if second != nil {
		t.Fatal("second acquire unexpectedly returned a lock")
	}
	if !errors.Is(err, ErrActivated) {
		t.Fatalf("expected ErrActivated, got %v", err)
	}

	select {
	case <-first.Activations():
	case <-time.After(time.Second):
		t.Fatal("existing instance did not receive activation")
	}
}
