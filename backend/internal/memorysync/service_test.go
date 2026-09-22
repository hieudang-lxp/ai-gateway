package memorysync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryBatchesRespectCountAndEncodedBytes(t *testing.T) {
	var received int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/status" {
			w.Write([]byte(`{"version":1,"ready":true,"capabilities":{"ingest":true}}`))
			return
		}
		if r.ContentLength > 1024*1024 {
			t.Errorf("oversized encoded request: %d", r.ContentLength)
		}
		var body struct {
			Messages []Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if len(body.Messages) > 100 {
			t.Error("batch exceeds 100")
		}
		received += len(body.Messages)
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]int{"accepted": len(body.Messages), "duplicates": 0})
	}))
	defer server.Close()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var messages []Message
	for i := 0; i < 205; i++ {
		m := fixtureMessage()
		m.UUID = fmt.Sprint(i)
		if i < 12 {
			m.Text = strings.Repeat("<", 100*1024)
		}
		messages = append(messages, m)
	}
	worker := NewWorker(Config{URL: server.URL}, db, &fixtureSource{messages: messages})
	worker.Sync(context.Background())
	if received != 205 || worker.Status().Pending != 0 {
		t.Fatalf("delivery lost messages received=%d status=%+v", received, worker.Status())
	}
}

func TestInvalidAcceptanceAndAuthenticationErrorsNeverCheckpoint(t *testing.T) {
	for _, response := range []struct {
		code int
		body string
	}{{202, `{"accepted":0,"duplicates":0}`}, {202, `{"accepted":1}`}, {401, "private response details"}, {503, "outage"}} {
		t.Run(fmt.Sprint(response.code, response.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/status" {
					w.Write([]byte(`{"version":1,"ready":true,"capabilities":{"ingest":true}}`))
					return
				}
				w.WriteHeader(response.code)
				w.Write([]byte(response.body))
			}))
			defer server.Close()
			db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			worker := NewWorker(Config{URL: server.URL}, db, &fixtureSource{messages: []Message{fixtureMessage()}})
			worker.Sync(context.Background())
			if worker.Status().Pending != 1 || worker.Status().Delivered != 0 || strings.Contains(worker.Status().Error, "private response") {
				t.Fatalf("wrong acceptance state: %+v", worker.Status())
			}
		})
	}
}

func TestHealthDoesNotDependOnReceiverAndStatusDoesNotExposeText(t *testing.T) {
	w := NewWorker(Config{}, nil, &fixtureSource{messages: []Message{fixtureMessage()}})
	for _, path := range []string{"/healthz", "/_memory"} {
		response := httptest.NewRecorder()
		w.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 200 {
			t.Fatalf("disabled health failed %s", path)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w = NewWorker(Config{URL: server.URL, Token: "private-secret"}, db, &fixtureSource{messages: []Message{fixtureMessage()}})
	w.Sync(context.Background())
	response := httptest.NewRecorder()
	w.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != 200 {
		t.Fatal("receiver outage broke health")
	}
	response = httptest.NewRecorder()
	w.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_memory", nil))
	if strings.Contains(response.Body.String(), "private-secret") || strings.Contains(response.Body.String(), `"text"`) {
		t.Fatal("status exposes content or credentials")
	}
}

func TestEquivalentPayloadFormattingDoesNotCreateIdentityConflict(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := fixtureMessage()
	if err = db.Enqueue(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	m.Text = "  ok\n"
	m.Timestamp = "2026-09-22T08:00:00+07:00"
	if err = db.Enqueue(context.Background(), m); err != nil {
		t.Fatalf("equivalent payload caused false conflict: %v", err)
	}
}

func TestPermanentRejectionAndOversizeDoNotBlockOtherMessages(t *testing.T) {
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/status" {
			w.Write([]byte(`{"version":1,"ready":true,"capabilities":{"ingest":true}}`))
			return
		}
		var body struct {
			Messages []Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			if m.UUID == "conflict" {
				w.WriteHeader(409)
				return
			}
			if len(m.Text) > 256*1024 {
				t.Error("oversize message sent")
			}
		}
		for _, m := range body.Messages {
			received = append(received, m.UUID)
		}
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]int{"accepted": len(body.Messages), "duplicates": 0})
	}))
	defer server.Close()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conflict := fixtureMessage()
	conflict.UUID = "conflict"
	oversize := fixtureMessage()
	oversize.UUID = "huge"
	oversize.Text = strings.Repeat("x", 256*1024+1)
	worker := NewWorker(Config{URL: server.URL}, db, &fixtureSource{messages: []Message{conflict, oversize, fixtureMessage()}})
	worker.Sync(context.Background())
	worker.Sync(context.Background())
	status := worker.Status()
	if len(received) != 1 || received[0] != "u1" || status.Pending != 2 || status.Delivered != 1 || status.State != "error" {
		t.Fatalf("poison blocked valid delivery: received=%v status=%+v", received, status)
	}
}

type fixtureSource struct {
	calls    int
	messages []Message
}

func (s *fixtureSource) Name() string { return "claude_code" }
func (s *fixtureSource) Scan(ctx context.Context, emit func(Message) error) error {
	s.calls++
	for _, m := range s.messages {
		if err := emit(m); err != nil {
			return err
		}
	}
	return nil
}
func fixtureMessage() Message {
	return Message{Source: "claude_code", SessionID: "s1", UUID: "u1", Speaker: "user", Text: "ok", Timestamp: "2026-09-22T01:00:00Z", GroupID: "personal"}
}

func TestDisabledAndUnavailableNeverReadSources(t *testing.T) {
	s := &fixtureSource{messages: []Message{fixtureMessage()}}
	worker := NewWorker(Config{}, nil, s)
	worker.Sync(context.Background())
	if s.calls != 0 || worker.Status().State != "disabled" {
		t.Fatal("disabled worker read transcripts")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"version": 1, "ready": false, "capabilities": map[string]bool{"ingest": true}})
	}))
	defer server.Close()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	worker = NewWorker(Config{URL: server.URL}, store, s)
	worker.Sync(context.Background())
	if s.calls != 0 || worker.Status().State != "waiting" {
		t.Fatal("unready worker read transcripts")
	}
}

func TestDeliveryReplayAndRestart(t *testing.T) {
	var requests int
	fail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing auth")
		}
		if r.URL.Path == "/v1/status" {
			w.Write([]byte(`{"version":1,"ready":true,"capabilities":{"ingest":true}}`))
			return
		}
		if r.URL.Path != "/v1/episodes" {
			t.Error("wrong route")
		}
		var body struct {
			Version  int       `json:"version"`
			Messages []Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Version != 1 || len(body.Messages) != 1 || body.Messages[0] != fixtureMessage() {
			t.Errorf("wrong body %+v", body)
		}
		requests++
		if fail {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"accepted":0,"duplicates":1}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "sender.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &fixtureSource{messages: []Message{fixtureMessage()}}
	worker := NewWorker(Config{URL: server.URL, Token: "secret"}, db, s)
	worker.Sync(context.Background())
	if requests != 1 || worker.Status().Pending != 1 || worker.Status().Delivered != 0 {
		t.Fatalf("failure lost durable message %+v", worker.Status())
	}
	db.Close()
	fail = false
	db, err = OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker = NewWorker(Config{URL: server.URL, Token: "secret"}, db, s)
	worker.Sync(context.Background())
	worker.Sync(context.Background())
	if requests != 2 || worker.Status().Pending != 0 || worker.Status().Delivered != 1 {
		t.Fatalf("restart/replay broken %+v requests=%d", worker.Status(), requests)
	}
}

func TestReceiverConflictStaysPendingAndVisible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/status" {
			w.Write([]byte(`{"version":1,"ready":true,"capabilities":{"ingest":true}}`))
			return
		}
		w.WriteHeader(409)
	}))
	defer server.Close()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := NewWorker(Config{URL: server.URL}, db, &fixtureSource{messages: []Message{fixtureMessage()}})
	worker.Sync(context.Background())
	status := worker.Status()
	if status.Pending != 1 || status.Delivered != 0 || status.State != "error" || status.Error != "Graphiti ingestion HTTP 409: identity conflict; pending messages retained" {
		t.Fatalf("wrong conflict state %+v", status)
	}
}

func TestChangedLocalIdentityDoesNotReplacePendingPayload(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := fixtureMessage()
	if err = db.Enqueue(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	m.Text = "changed"
	if err = db.Enqueue(context.Background(), m); err == nil {
		t.Fatal("identity mutation accepted")
	}
	pending, err := db.Pending(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Text != "ok" {
		t.Fatal("original message was overwritten")
	}
}
