package memorysync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// CursorSource reads the modern local Cursor conversation database. It never
// uses the usage API or emits editor drafts, attachments, or assistant text.
type CursorSource struct{ path string }

func NewCursorSource(path string) *CursorSource { return &CursorSource{path: path} }
func (*CursorSource) Name() string              { return "cursor" }

type cursorComposer struct {
	ComposerID   string              `json:"composerId"`
	SubagentInfo *cursorSubagentInfo `json:"subagentInfo"`
}

type cursorSubagentInfo struct {
	ParentComposerID string `json:"parentComposerId"`
}

type cursorBubble struct {
	BubbleID       string `json:"bubbleId"`
	Type           int    `json:"type"`
	Text           string `json:"text"`
	CreatedAt      string `json:"createdAt"`
	IsSimulatedMsg bool   `json:"isSimulatedMsg"`
}

func (s *CursorSource) Scan(ctx context.Context, emit func(Message) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("cursor storage missing")
	}
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("cursor storage unavailable")
	}
	path, err := filepath.Abs(s.path)
	if err != nil {
		return errors.New("cursor storage path unavailable")
	}
	// mode=ro includes committed WAL records. immutable=1 would silently ignore
	// WAL updates in a running Cursor instance and must not be used here.
	uri := url.URL{Scheme: "file", Path: path}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "query_only(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return errors.New("cursor storage unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return errors.New("cursor storage read failed")
	}
	defer tx.Rollback()
	var tableCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='cursorDiskKV'`).Scan(&tableCount); err != nil {
		return errors.New("cursor storage schema read failed")
	}
	if tableCount == 0 {
		return errors.New("cursor storage unsupported: modern cursorDiskKV table missing; legacy workspace chat is not supported")
	}

	rows, err := tx.QueryContext(ctx, `SELECT key,value FROM cursorDiskKV WHERE key LIKE 'composerData:%' ORDER BY key`)
	if err != nil {
		return errors.New("cursor storage unsupported: composer records unavailable")
	}
	composers := make(map[string]cursorComposer)
	invalid := 0
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			rows.Close()
			return errors.New("cursor composer read failed")
		}
		var composer cursorComposer
		id := strings.TrimPrefix(key, "composerData:")
		if json.Unmarshal(value, &composer) != nil || id == "" || composer.ComposerID != id {
			invalid++
			continue
		}
		if composer.SubagentInfo != nil && composer.SubagentInfo.ParentComposerID == "" {
			invalid++
			continue
		}
		composers[id] = composer
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.New("cursor composer read failed")
	}

	// Scan persisted bubbles rather than only current conversation headers:
	// older branches can retain human messages outside the active header list.
	rows, err = tx.QueryContext(ctx, `SELECT key,value FROM cursorDiskKV WHERE key LIKE 'bubbleId:%' ORDER BY key`)
	if err != nil {
		return errors.New("cursor storage unsupported: bubble records unavailable")
	}
	defer rows.Close()
	bubbles := 0
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return errors.New("cursor bubble read failed")
		}
		bubbles++
		var bubble cursorBubble
		if json.Unmarshal(value, &bubble) != nil {
			invalid++
			continue
		}
		if bubble.Type == 2 {
			continue
		}
		if bubble.Type != 1 {
			invalid++
			continue
		}
		if bubble.IsSimulatedMsg {
			continue
		}
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 || parts[1] == "" || parts[2] == "" || bubble.BubbleID != parts[2] {
			invalid++
			continue
		}
		composer, ok := composers[parts[1]]
		if !ok {
			invalid++
			continue
		}
		// Subagent composers store model-generated task prompts as type=1, not
		// human input. Cursor records their ancestry explicitly in subagentInfo.
		if composer.SubagentInfo != nil {
			continue
		}
		if strings.TrimSpace(bubble.Text) == "" {
			continue
		}
		created, err := time.Parse(time.RFC3339Nano, bubble.CreatedAt)
		if err != nil {
			invalid++
			continue
		}
		if err := emit(Message{Source: "cursor", SessionID: parts[1], UUID: bubble.BubbleID, Speaker: "user", Text: bubble.Text, Timestamp: created.UTC().Format(time.RFC3339Nano), GroupID: "personal"}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.New("cursor bubble read failed")
	}
	if invalid > 0 {
		return fmt.Errorf("cursor storage unsupported or incomplete: %d invalid conversation records", invalid)
	}
	if len(composers) == 0 && bubbles == 0 {
		return errors.New("cursor storage unsupported or empty: no recognized conversations")
	}
	return nil
}
