package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/nats-io/nats.go"
)

func TestNamedConsumersFanOutAndResumeDurableCursor(t *testing.T) {
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("set TEST_NATS_URL to an isolated test NATS server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	nc, js, err := Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	suffix := uuid.NewString()
	sessionsName, insightsName := "sessions-test-"+suffix, "insights-test-"+suffix

	for _, name := range []string{sessionsName, insightsName} {
		t.Cleanup(func() {
			if err := js.DeleteConsumer(events.Stream, name); err != nil && !errors.Is(err, nats.ErrConsumerNotFound) {
				t.Errorf("delete consumer %s: %v", name, err)
			}
		})
	}
	firstID, missedID := uuid.NewString(), uuid.NewString()
	type running struct {
		cancel context.CancelFunc
		done   chan struct{}
		err    error
	}
	start := func(name string, apply func(context.Context, events.Envelope) error) *running {
		runCtx, stop := context.WithCancel(ctx)
		r := &running{cancel: stop, done: make(chan struct{})}
		go func() {
			defer close(r.done)
			r.err = consumeNamed(runCtx, js, name, func(e events.Envelope) error {

				if e.ID != firstID && e.ID != missedID {
					return nil
				}
				return apply(runCtx, e)
			}, func(_, _, reason string) error {
				return fmt.Errorf("unexpected invalid event: %s", reason)
			})
		}()
		t.Cleanup(func() {
			stop()
			select {
			case <-r.done:
			case <-time.After(5 * time.Second):
				t.Errorf("consumer %s did not stop", name)
			}
		})
		return r
	}
	awaitDelivery := func(r *running, deliveries <-chan string, want string) {
		t.Helper()
		select {
		case got := <-deliveries:
			if got != want {
				t.Fatalf("received %s, want %s", got, want)
			}
		case <-r.done:
			t.Fatalf("consumer stopped before delivery: %v", r.err)
		case <-ctx.Done():
			t.Fatalf("waiting for delivery: %v", ctx.Err())
		}
	}
	publish := func(id string) uint64 {
		t.Helper()
		payload, err := json.Marshal(events.Envelope{Version: 1, ID: id, Rows: []events.ExternalUsage{{
			Source: "codex", ID: "response-" + id, TS: time.Now().UTC(), Model: "test-model", Usage: events.Usage{Input: 10},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		ack, err := js.Publish(events.Subject, payload, nats.MsgId(id), nats.Context(ctx))
		if err != nil {
			t.Fatal(err)
		}
		return ack.Sequence
	}
	awaitAck := func(name string, sequence uint64) {
		t.Helper()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			info, err := js.ConsumerInfo(events.Stream, name, nats.Context(ctx))
			if err != nil {
				t.Fatal(err)
			}
			if info.AckFloor.Stream >= sequence {
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatalf("waiting for %s acknowledgment: %v", name, ctx.Err())
			}
		}
	}

	sessionDeliveries, insightDeliveries := make(chan string, 4), make(chan string, 4)
	releaseApply := make(chan struct{})
	sessions := start(sessionsName, func(ctx context.Context, e events.Envelope) error {
		sessionDeliveries <- e.ID
		select {
		case <-releaseApply:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	insights := start(insightsName, func(_ context.Context, e events.Envelope) error {
		insightDeliveries <- e.ID
		return nil
	})
	firstSequence := publish(firstID)
	awaitDelivery(sessions, sessionDeliveries, firstID)
	awaitDelivery(insights, insightDeliveries, firstID)
	awaitAck(insightsName, firstSequence)
	info, err := js.ConsumerInfo(events.Stream, sessionsName, nats.Context(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if info.NumAckPending != 1 || info.AckFloor.Stream >= firstSequence {
		t.Fatalf("event acknowledged before apply finished: pending=%d ack_floor=%d event=%d", info.NumAckPending, info.AckFloor.Stream, firstSequence)
	}
	close(releaseApply)
	awaitAck(sessionsName, firstSequence)
	sessions.cancel()
	select {
	case <-sessions.done:
		if !errors.Is(sessions.err, context.Canceled) {
			t.Fatalf("stopping sessions consumer: %v", sessions.err)
		}
	case <-ctx.Done():
		t.Fatalf("stopping sessions consumer: %v", ctx.Err())
	}

	missedSequence := publish(missedID)
	awaitDelivery(insights, insightDeliveries, missedID)
	awaitAck(insightsName, missedSequence)
	resumed := start(sessionsName, func(_ context.Context, e events.Envelope) error {
		sessionDeliveries <- e.ID
		return nil
	})

	awaitDelivery(resumed, sessionDeliveries, missedID)
	awaitAck(sessionsName, missedSequence)
}
