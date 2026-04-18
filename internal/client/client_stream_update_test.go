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

func TestUpdateStream_SuccessfulPUT(t *testing.T) {
	path := "/api/streams/str1"
	var putCalled bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}
		if r.Method != http.MethodPut {
			w.WriteHeader(405)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
			return
		}
		putCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "str1", "title": "updated"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	s := &Stream{Title: "updated"}
	out, err := c.UpdateStream("str1", s)
	if err != nil {
		t.Fatalf("UpdateStream error: %v", err)
	}
	if !putCalled {
		t.Fatal("expected PUT to be called")
	}
	if out.ID != "str1" || out.Title != "updated" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestUpdateStream_ErrorOn405(t *testing.T) {
	path := "/api/streams/str1"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(404)
			return
		}
		// Всегда возвращаем 405
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "method not allowed"})
	}))
	defer ts.Close()

	c := newStreamTestClient(ts.URL)
	s := &Stream{Title: "test"}
	_, err := c.UpdateStream("str1", s)
	if err == nil {
		t.Fatal("expected error on 405 response")
	}

	ge, ok := err.(*GraylogError)
	if !ok {
		t.Fatalf("expected GraylogError, got %T", err)
	}
	if ge.Status != 405 {
		t.Errorf("expected status 405, got %d", ge.Status)
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
