package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newStreamTestClient(base string) *Client {
	return &Client{
		BaseURL:    base,
		HTTP:       &http.Client{Timeout: 2 * time.Second},
		MaxRetries: 0,
		RetryWait:  time.Millisecond,
		logger:     NoopLogger{},
		APIVersion: APIV7,
	}
}

func TestUpdateStream_MethodFallbacks(t *testing.T) {
	path := "/api/streams/str1"
	calls := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}
		calls = append(calls, r.Method)
		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "str1", "title": "after"})
		default:
			w.WriteHeader(405)
		}
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	s := &Stream{Title: "before"}
	out, err := c.UpdateStream("str1", s)
	if err != nil {
		t.Fatalf("UpdateStream: %v", err)
	}
	if out.ID != "str1" || out.Title != "after" {
		t.Fatalf("unexpected out: %+v", out)
	}
	// допускаем финальный GET (клиент перечитывает ресурс для v7)
	wantPrefix := []string{"PUT", "PATCH", "POST"}
	if len(calls) < len(wantPrefix) {
		t.Fatalf("methods used: %v, want prefix %v", calls, wantPrefix)
	}
	for i := range wantPrefix {
		if calls[i] != wantPrefix[i] {
			t.Fatalf("method sequence mismatch: %v", calls)
		}
	}
}

func TestCreateStream_V7AndLegacyBodies(t *testing.T) {
	// Первая попытка — v7 entity wrapper (вернём ошибку), затем — legacy (вернём успех)
	path := "/api/streams"
	attempt := 0
	var lastBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		attempt++
		lastBody = map[string]any{} // очистить, чтобы ключи прошлого запроса не сохранились
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		if attempt == 1 {
			// эмулируем 400 на v7-форму
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "bad request"})
			return
		}
		// Вторая попытка — должна быть legacy форма с полями верхнего уровня
		if _, ok := lastBody["entity"]; ok {
			t.Fatalf("expected legacy body on 2nd attempt, got v7 entity: %+v", lastBody)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"stream_id": "sid"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	s := &Stream{Title: "t", Description: "d", IndexSetID: "idx"}
	got, err := c.CreateStream(s)
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	if got.ID != "sid" {
		t.Fatalf("unexpected id: %+v", got)
	}
}
