package sse

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHub_SubscribeAndPublish(t *testing.T) {
	hub := NewHub()
	id := uuid.New()

	sub := hub.Subscribe(id)
	hub.Publish(id)

	select {
	case <-sub.C():
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected signal after publish")
	}
}

func TestHub_PublishCoalesces(t *testing.T) {
	hub := NewHub()
	id := uuid.New()

	sub := hub.Subscribe(id)

	// Publish twice rapidly before the subscriber reads
	hub.Publish(id)
	hub.Publish(id)

	// Should get exactly one signal (channel capacity is 1)
	select {
	case <-sub.C():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected signal")
	}

	// Channel should now be empty
	select {
	case <-sub.C():
		t.Fatal("expected no second signal (coalesced)")
	case <-time.After(50 * time.Millisecond):
		// correct — coalesced
	}
}

func TestHub_UnsubscribeCleansUp(t *testing.T) {
	hub := NewHub()
	id := uuid.New()

	sub := hub.Subscribe(id)
	if hub.Len() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", hub.Len())
	}

	hub.Unsubscribe(id, sub)
	if hub.Len() != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", hub.Len())
	}

	// Publish after unsubscribe should not panic or block
	hub.Publish(id)
}

func TestHub_PublishToMultipleSubscribers(t *testing.T) {
	hub := NewHub()
	id := uuid.New()

	sub1 := hub.Subscribe(id)
	sub2 := hub.Subscribe(id)

	hub.Publish(id)

	for i, sub := range []*Subscriber{sub1, sub2} {
		select {
		case <-sub.C():
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d did not receive signal", i+1)
		}
	}
}

func TestHub_PublishToWrongRequestID(t *testing.T) {
	hub := NewHub()
	idA := uuid.New()
	idB := uuid.New()

	sub := hub.Subscribe(idA)
	hub.Publish(idB) // publish for a different request

	select {
	case <-sub.C():
		t.Fatal("subscriber for ID-A should not receive signal from ID-B publish")
	case <-time.After(50 * time.Millisecond):
		// correct
	}
}

func TestHub_ConcurrentSubscribePublish(t *testing.T) {
	hub := NewHub()
	id := uuid.New()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Spawn concurrent subscribers
	subs := make([]*Subscriber, goroutines)
	var mu sync.Mutex
	for i := range goroutines {
		go func() {
			defer wg.Done()
			s := hub.Subscribe(id)
			mu.Lock()
			subs[i] = s
			mu.Unlock()
		}()
	}

	// Spawn concurrent publishers
	for range goroutines {
		go func() {
			defer wg.Done()
			hub.Publish(id)
		}()
	}

	wg.Wait()

	// All subscribers should have been registered
	if hub.Len() != goroutines {
		t.Fatalf("expected %d subscribers, got %d", goroutines, hub.Len())
	}

	// Clean up all
	for _, sub := range subs {
		hub.Unsubscribe(id, sub)
	}

	if hub.Len() != 0 {
		t.Fatalf("expected 0 subscribers after cleanup, got %d", hub.Len())
	}
}

func TestHub_UnsubscribeIdempotent(t *testing.T) {
	hub := NewHub()
	id := uuid.New()

	sub := hub.Subscribe(id)
	hub.Unsubscribe(id, sub)
	// Second unsubscribe should not panic
	hub.Unsubscribe(id, sub)

	if hub.Len() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", hub.Len())
	}
}
