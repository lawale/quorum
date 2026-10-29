package service

import (
	"context"
	"sync"

	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// fakeSink records every event a service emits and how often it was woken.
type fakeSink struct {
	mu      sync.Mutex
	events  []model.Event
	signals int
}

func (f *fakeSink) Enqueue(_ context.Context, _ *store.Stores, ev model.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := *ev.Request
	ev.Request = &copied
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeSink) Signal() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals++
}

// named returns the events with the given name.
func (f *fakeSink) named(name string) []model.Event {
	var out []model.Event
	for _, e := range f.events {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}
