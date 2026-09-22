package memorysync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	URL, Token, GroupID string
	Interval            time.Duration
	Client              *http.Client
}
type SourceStatus struct {
	State       string     `json:"state"`
	Error       string     `json:"error,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	Pending     int        `json:"pending"`
	Delivered   int        `json:"delivered"`
	Blocked     int        `json:"blocked"`
}
type Status struct {
	Enabled   bool                    `json:"enabled"`
	State     string                  `json:"state"`
	Ready     bool                    `json:"ready"`
	Error     string                  `json:"error,omitempty"`
	Pending   int                     `json:"pending"`
	Delivered int                     `json:"delivered"`
	Blocked   int                     `json:"blocked"`
	LastCheck *time.Time              `json:"last_check,omitempty"`
	Sources   map[string]SourceStatus `json:"sources"`
	// Receiver status is metadata only; acceptance does not imply extraction.
	Receiver json.RawMessage `json:"receiver,omitempty"`
}
type Worker struct {
	config  Config
	store   *Store
	sources []Source
	client  *http.Client
	mu      sync.RWMutex
	syncMu  sync.Mutex
	status  Status
}

func NewWorker(config Config, store *Store, sources ...Source) *Worker {
	config.URL = strings.TrimRight(strings.TrimSpace(config.URL), "/")
	if config.GroupID == "" {
		config.GroupID = "personal"
	}
	if config.Interval <= 0 {
		config.Interval = 30 * time.Second
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// A receiver redirect must never forward credentials or transcript content.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	state := "waiting"
	if config.URL == "" {
		state = "disabled"
	}
	w := &Worker{config: config, store: store, sources: sources, client: &copyClient, status: Status{Enabled: config.URL != "", State: state, Sources: map[string]SourceStatus{}}}
	for _, source := range sources {
		w.status.Sources[source.Name()] = SourceStatus{State: state}
	}
	return w
}

func (w *Worker) Run(ctx context.Context) {
	go func() {
		w.Sync(ctx)
		ticker := time.NewTicker(w.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.Sync(ctx)
			}
		}
	}()
}
func (w *Worker) Status() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := w.status
	out.Sources = map[string]SourceStatus{}
	for k, v := range w.status.Sources {
		out.Sources[k] = v
	}
	out.Receiver = append(json.RawMessage(nil), w.status.Receiver...)
	return out
}
func (w *Worker) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	parsed, err := url.Parse(w.config.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("GRAPHITI_URL must be an HTTP(S) URL without credentials")
	}
	req, err := http.NewRequestWithContext(ctx, method, w.config.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid Graphiti request URL")
	}
	if w.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+w.config.Token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := w.client.Do(req)
	if err != nil {
		return nil, errors.New("Graphiti API unreachable or request timed out")
	}
	return response, nil
}
func (w *Worker) checkReady(ctx context.Context) (json.RawMessage, error) {
	response, err := w.request(ctx, http.MethodGet, "/v1/status", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Graphiti readiness HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return nil, errors.New("Graphiti readiness response unreadable")
	}
	var status struct {
		Version      int  `json:"version"`
		Ready        bool `json:"ready"`
		Capabilities struct {
			Ingest bool `json:"ingest"`
		} `json:"capabilities"`
		Dependencies map[string]bool `json:"dependencies"`
		Queue        map[string]int  `json:"queue"`
	}
	if json.Unmarshal(body, &status) != nil || status.Version != 1 {
		return nil, errors.New("unsupported Graphiti readiness response")
	}
	// Re-encode only specified metadata, never expose arbitrary receiver body.
	safe, _ := json.Marshal(status)
	if !status.Ready || !status.Capabilities.Ingest {
		return safe, errors.New("Graphiti extraction dependencies are not ready")
	}
	return safe, nil
}

func (w *Worker) Sync(ctx context.Context) {
	w.syncMu.Lock()
	defer w.syncMu.Unlock()
	if w.config.URL == "" {
		return
	}
	snapshot := w.Status()
	now := time.Now().UTC()
	snapshot.LastCheck = &now
	snapshot.Error = ""
	snapshot.State = "waiting"
	snapshot.Ready = false
	defer func() { w.mu.Lock(); w.status = snapshot; w.mu.Unlock() }()
	if w.store == nil {
		snapshot.State = "error"
		snapshot.Error = "sender database not configured"
		return
	}
	defer func() {
		counts, err := w.store.Counts(ctx)
		if err != nil {
			snapshot.State = "error"
			snapshot.Error = err.Error()
			return
		}
		snapshot.Pending = 0
		snapshot.Delivered = 0
		snapshot.Blocked = 0
		for name, s := range snapshot.Sources {
			c := counts[name]
			s.Pending = c.Pending
			s.Delivered = c.Delivered
			s.Blocked = c.Blocked
			if c.Blocked > 0 {
				s.State = "error"
				s.Error = c.Error
				snapshot.State = "error"
				if snapshot.Error == "" {
					snapshot.Error = c.Error
				}
			}
			snapshot.Sources[name] = s
		}
		for _, c := range counts {
			snapshot.Pending += c.Pending
			snapshot.Delivered += c.Delivered
			snapshot.Blocked += c.Blocked
		}
	}()
	receiver, err := w.checkReady(ctx)
	snapshot.Receiver = receiver
	if err != nil {
		snapshot.Error = err.Error()
		for name, s := range snapshot.Sources {
			s.State = "waiting"
			s.Error = "delivery paused: " + err.Error()
			snapshot.Sources[name] = s
		}
		return
	}
	snapshot.Ready = true
	snapshot.State = "ok"
	for _, source := range w.sources {
		s := snapshot.Sources[source.Name()]
		err = source.Scan(ctx, func(m Message) error {
			m.GroupID = w.config.GroupID
			m.Speaker = "user"
			return w.store.Enqueue(ctx, m)
		})
		if err != nil {
			s.State = "error"
			s.Error = err.Error()
			snapshot.State = "error"
		} else {
			s.State = "ok"
			s.Error = ""
			checked := time.Now().UTC()
			s.LastSuccess = &checked
		}
		snapshot.Sources[source.Name()] = s
	}
	for ctx.Err() == nil {
		messages, err := w.store.Pending(ctx, 100)
		if err != nil {
			snapshot.State = "error"
			snapshot.Error = err.Error()
			return
		}
		if len(messages) == 0 {
			return
		}
		valid := messages[:0]
		for _, message := range messages {
			if len(message.Text) > 256*1024 {
				if err := w.store.Block(ctx, message, "message exceeds 256 KiB text limit; pending message retained"); err != nil {
					snapshot.State = "error"
					snapshot.Error = err.Error()
					return
				}
			} else {
				valid = append(valid, message)
			}
		}
		messages = valid
		if len(messages) == 0 {
			continue
		}
		// Keep request bodies bounded as well as message count. Oversized records
		// remain pending with an explicit error rather than being silently dropped.
		body, _ := json.Marshal(struct {
			Version  int       `json:"version"`
			Messages []Message `json:"messages"`
		}{1, messages})
		for len(body) > 1024*1024 && len(messages) > 1 {
			messages = messages[:len(messages)-1]
			body, _ = json.Marshal(struct {
				Version  int       `json:"version"`
				Messages []Message `json:"messages"`
			}{1, messages})
		}
		if len(body) > 1024*1024 {
			if err := w.store.Block(ctx, messages[0], "message exceeds 1 MiB encoded ingestion limit; pending message retained"); err != nil {
				snapshot.State = "error"
				snapshot.Error = err.Error()
				return
			}
			continue
		}
		if err = w.deliver(ctx, body, len(messages)); err != nil {
			var rejected *rejection
			if errors.As(err, &rejected) {
				// A batch rejection is atomic. Isolate each record to quarantine
				// only offenders, letting other sources and messages progress.
				for _, m := range messages {
					one, _ := json.Marshal(struct {
						Version  int       `json:"version"`
						Messages []Message `json:"messages"`
					}{1, []Message{m}})
					singleErr := err
					if len(messages) > 1 {
						singleErr = w.deliver(ctx, one, 1)
					}
					if singleErr == nil {
						singleErr = w.store.Delivered(ctx, []Message{m})
					} else if errors.As(singleErr, &rejected) {
						singleErr = w.store.Block(ctx, m, singleErr.Error())
					}
					if singleErr != nil {
						snapshot.State = "error"
						snapshot.Error = singleErr.Error()
						return
					}
				}
				continue
			}
			snapshot.State = "error"
			snapshot.Error = err.Error()
			return
		}
		if err = w.store.Delivered(ctx, messages); err != nil {
			snapshot.State = "error"
			snapshot.Error = err.Error()
			return
		}
	}
}

type rejection struct{ status int }

func (e *rejection) Error() string {
	if e.status == 409 {
		return "Graphiti ingestion HTTP 409: identity conflict; pending messages retained"
	}
	return fmt.Sprintf("Graphiti ingestion HTTP %d: rejected message; pending messages retained", e.status)
}

func (w *Worker) deliver(ctx context.Context, body []byte, count int) error {
	response, err := w.request(ctx, http.MethodPost, "/v1/episodes", body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 400 || response.StatusCode == 409 || response.StatusCode == 413 {
		return &rejection{status: response.StatusCode}
	}
	if response.StatusCode != 202 {
		return fmt.Errorf("Graphiti ingestion HTTP %d; pending messages retained", response.StatusCode)
	}
	var result struct {
		Accepted   *int `json:"accepted"`
		Duplicates *int `json:"duplicates"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&result) != nil || result.Accepted == nil || result.Duplicates == nil || *result.Accepted < 0 || *result.Duplicates < 0 || *result.Accepted+*result.Duplicates != count {
		return errors.New("invalid Graphiti acceptance response; pending messages retained")
	}
	return nil
}

func (w *Worker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_memory", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		json.NewEncoder(response).Encode(w.Status())
	})
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, request *http.Request) {
		if w.config.URL != "" {
			if w.store == nil {
				http.Error(response, "sender database unavailable", 503)
				return
			}
			if _, err := w.store.Counts(request.Context()); err != nil {
				http.Error(response, "sender database unavailable", 503)
				return
			}
		}
		response.Header().Set("Content-Type", "application/json")
		json.NewEncoder(response).Encode(map[string]any{"status": "ok", "enabled": w.config.URL != ""})
	})
	return mux
}
