package eventbus

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/nats-io/nats.go"
	"log"
	"time"
)

func Consume(ctx context.Context, url string, apply func(events.Envelope) error, quarantine func(string, string, string) error) {
	for ctx.Err() == nil {
		nc, js, err := Connect(url)
		if err == nil {
			err = consume(ctx, js, apply, quarantine)
			nc.Close()
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("usage consumer retry: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func consume(ctx context.Context, js nats.JetStreamContext, apply func(events.Envelope) error, quarantine func(string, string, string) error) error {
	// A single ordered consumer preserves collector batch/status order. Separate
	// subscribers for future services get independent durable consumers.
	_, err := js.AddConsumer(events.Stream, &nats.ConsumerConfig{Durable: "usage-ledger-v1", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: 60 * time.Second})
	if err != nil {
		return err
	}
	sub, err := js.PullSubscribe("", "usage-ledger-v1", nats.Bind(events.Stream, "usage-ledger-v1"))
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	for ctx.Err() == nil {
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			return err
		}
		for _, msg := range msgs {
			if err = process(msg.Data, apply, func(reason string) error {
				hash := fmt.Sprintf("%x", sha256.Sum256(msg.Data))
				if err := quarantine(hash, msg.Subject, reason); err != nil {
					return err
				}
				log.Printf("quarantined invalid event on %s: %s", msg.Subject, reason)
				return nil
			}); err != nil {
				return err
			}
			if err = msg.AckSync(); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func process(payload []byte, apply func(events.Envelope) error, quarantine func(string) error) error {
	var e events.Envelope
	if err := json.Unmarshal(payload, &e); err != nil {
		return quarantine("invalid event JSON")
	}
	err := events.Validate(e)
	if err == nil {
		err = apply(e)
	}
	if errors.Is(err, events.ErrInvalid) {
		return quarantine(err.Error())
	}
	return err // database/network failures remain unacknowledged for retry
}
