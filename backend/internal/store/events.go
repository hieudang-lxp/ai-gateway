package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hieudang-lxp/ai-gateway/backend/contracts/events"
	"github.com/hieudang-lxp/ai-gateway/backend/internal/eventbus"
)

func (s *Store) callEvent(c Call) events.ExternalUsage {
	cost := c.CostUSD
	return events.ExternalUsage{Source: "claude_gateway", ID: fmt.Sprintf("%s:%d", s.eventOrigin, c.ID), TS: c.TS, Model: c.Model, Usage: c.Usage, CostUSD: &cost, CostKind: "estimated"}
}

// EnableEvents backfills old proxy calls once and enables transactional outbox
// writes for new calls. Invoke before accepting requests.
func (s *Store) EnableEvents() error {
	outboxSchema := eventbus.Schema
	if s.postgres {
		outboxSchema = eventbus.PostgresSchema
	}
	if _, err := s.db.Exec(outboxSchema + `CREATE TABLE IF NOT EXISTS gateway_event_state(id INTEGER PRIMARY KEY CHECK(id=1),origin TEXT NOT NULL,last_id BIGINT NOT NULL);`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO gateway_event_state VALUES(1,$1,0) ON CONFLICT DO NOTHING`, uuid.NewString()); err != nil {
		return err
	}
	var after int64
	if err := s.db.QueryRow(`SELECT origin,last_id FROM gateway_event_state WHERE id=1`).Scan(&s.eventOrigin, &after); err != nil {
		return err
	}
	for {
		calls, err := s.CallsAfter(after, 100)
		if err != nil {
			return err
		}
		if len(calls) == 0 {
			return nil
		}
		rows := make([]events.ExternalUsage, 0, len(calls))
		for _, c := range calls {
			rows = append(rows, s.callEvent(c))
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if err = eventbus.Enqueue(tx, events.Subject, events.Envelope{Rows: rows}); err != nil {
			tx.Rollback()
			return err
		}
		after = calls[len(calls)-1].ID
		if _, err = tx.Exec(`UPDATE gateway_event_state SET last_id=$1 WHERE id=1`, after); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
}
func (s *Store) PublishEvents(ctx context.Context, url string) { eventbus.Wrap(s.db).Pump(ctx, url) }
