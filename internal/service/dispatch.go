package service

import (
	"context"

	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// EventSink receives request events on the caller's transaction and is woken
// once that transaction commits. The webhook dispatcher satisfies it; the
// service never imports the webhook package.
type EventSink interface {
	Enqueue(ctx context.Context, tx *store.Stores, ev model.Event) error
	Signal()
}

// txRunner executes fn inside a database transaction, handing it stores bound
// to that transaction. A nil error commits; anything else rolls back.
type txRunner = func(ctx context.Context, fn func(tx *store.Stores) error) error

// emitFunc queues an event on the transaction currently in flight.
type emitFunc func(model.Event) error

// dispatch is how a service changes state and announces it. Every state
// change goes through transact, which guarantees the announcement is written
// on the same transaction as the change and the delivery worker is woken only
// after that transaction commits.
type dispatch struct {
	runInTx txRunner
	sink    EventSink
}

// transact runs fn in a transaction. fn receives the transaction-bound stores
// and an emit that writes events on that same transaction, so a crash can
// never leave a state change unannounced. If fn emitted anything and the
// transaction committed, the sink is signalled. Without a sink, emit is a
// no-op and nothing is signalled.
func (d dispatch) transact(ctx context.Context, fn func(tx *store.Stores, emit emitFunc) error) error {
	emitted := false
	err := d.runInTx(ctx, func(tx *store.Stores) error {
		return fn(tx, func(ev model.Event) error {
			if d.sink == nil {
				return nil
			}
			emitted = true
			return d.sink.Enqueue(ctx, tx, ev)
		})
	})
	if err == nil && emitted {
		d.sink.Signal()
	}
	return err
}

// passThrough returns a runner that hands fn the given stores with no
// transaction around them. It is the default until a real runner is
// installed, so code paths never have to branch on whether one exists.
func passThrough(stores *store.Stores) txRunner {
	return func(ctx context.Context, fn func(tx *store.Stores) error) error {
		return fn(stores)
	}
}
