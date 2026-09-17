package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// userServer records every request as "METHOD path" (plus the body of the
// last write) and answers GET /api/users/<name> with an ObjectId, which the
// 6/7 code paths resolve before PUTting.
func userServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*Client, *[]string, *string) {
	t.Helper()
	var calls []string
	var lastBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// detectVersion probes both paths before any real call.
		if r.URL.Path == "/api/system" || r.URL.Path == "/system" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"7.0.0"}`))
			return
		}
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		if b, err := io.ReadAll(r.Body); err == nil && r.Method != http.MethodGet {
			lastBody = string(b)
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/users/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"id":"0123456789abcdef01234567","username":%q,"session_timeout_ms":28800000,"account_status":"enabled"}`, strings.TrimPrefix(r.URL.Path, "/api/users/"))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(ts.Close)

	c := New(ts.URL, "dGVzdA==")
	c.MaxRetries = 0
	return c, &calls, &lastBody
}

func countCalls(calls []string, suffix string) int {
	n := 0
	for _, c := range calls {
		if strings.HasSuffix(c, suffix) {
			n++
		}
	}
	return n
}

// Graylog accepts session_timeout_ms:0 on create but then rejects every
// interactive login with "Session timeout is set to 0 seconds", so an unset
// value must be left out of the request for the server default to apply.
func TestCreateUser_OmitsSessionTimeoutWhenUnset(t *testing.T) {
	c, _, gotBody := userServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	if _, err := c.CreateUser(&User{Username: "alice", FullName: "Alice Doe", Password: "pw"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(*gotBody), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if _, ok := body["session_timeout_ms"]; ok {
		t.Fatalf("session_timeout_ms must be omitted when unset, got body %s", *gotBody)
	}
}

func TestCreateUser_SendsSessionTimeoutWhenSet(t *testing.T) {
	c, _, gotBody := userServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	if _, err := c.CreateUser(&User{Username: "alice", FullName: "Alice Doe", Password: "pw", SessionTimeoutMs: 3600000}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(*gotBody), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got := body["session_timeout_ms"]; got != float64(3600000) {
		t.Fatalf("expected session_timeout_ms 3600000, got %v (body %s)", got, *gotBody)
	}
}

func TestSetUserDisabled_UsesStatusEndpointAndRefreshes(t *testing.T) {
	statusChanged := false
	c, calls, _ := userServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/api/users/0123456789abcdef01234567/status/disabled" {
			statusChanged = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	updated, err := c.SetUserDisabled("alice", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !statusChanged {
		t.Fatalf("status endpoint was not called: %v", *calls)
	}
	if updated.Username != "alice" {
		t.Fatalf("unexpected refreshed user: %+v", updated)
	}
	if n := countCalls(*calls, "/status/disabled"); n != 1 {
		t.Fatalf("expected one status update, got %d: %v", n, *calls)
	}
}

func TestSetUserDisabled_ResolvesObjectIDFromUserList(t *testing.T) {
	const objectID = "0123456789abcdef01234567"
	disabled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/system" || r.URL.Path == "/system":
			_, _ = w.Write([]byte(`{"version":"7.0.0"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/users/alice":
			status := "enabled"
			if disabled {
				status = "disabled"
			}
			// The detail endpoint deliberately omits id, matching the live API.
			_, _ = fmt.Fprintf(w, `{"username":"alice","session_timeout_ms":28800000,"account_status":%q}`, status)
		case r.Method == http.MethodGet && r.URL.Path == "/api/users":
			_, _ = fmt.Fprintf(w, `{"users":[{"id":%q,"username":"alice"}]}`, objectID)
		case r.Method == http.MethodPut && r.URL.Path == "/api/users/"+objectID+"/status/disabled":
			disabled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(ts.URL, "dGVzdA==")
	c.MaxRetries = 0
	updated, err := c.SetUserDisabled("alice", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.Disabled {
		t.Fatalf("expected refreshed user to be disabled: %+v", updated)
	}
}

func TestSetUserDisabled_V5UsesObjectIDStatusEndpointAndRefreshes(t *testing.T) {
	const objectID = "0123456789abcdef01234567"
	disabled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/system" || r.URL.Path == "/system":
			_, _ = w.Write([]byte(`{"version":"5.0.0"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/users/alice":
			status := "enabled"
			if disabled {
				status = "disabled"
			}
			_, _ = fmt.Fprintf(w, `{"username":"alice","session_timeout_ms":28800000,"account_status":%q}`, status)
		case r.Method == http.MethodGet && r.URL.Path == "/api/users":
			_, _ = fmt.Fprintf(w, `{"users":[{"id":%q,"username":"alice"}]}`, objectID)
		case r.Method == http.MethodPut && r.URL.Path == "/api/users/"+objectID+"/status/disabled":
			disabled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(ts.URL, "dGVzdA==")
	c.MaxRetries = 0
	updated, err := c.SetUserDisabled("alice", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.Disabled {
		t.Fatalf("expected refreshed Graylog 5 user to be disabled: %+v", updated)
	}
}

func TestSetUserDisabled_ReturnsFallbackFailure(t *testing.T) {
	c, calls, _ := userServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/status/disabled") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/disable") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := c.SetUserDisabled("alice", true); err == nil {
		t.Fatalf("expected both status endpoints to fail, calls: %v", *calls)
	}
	if countCalls(*calls, "/status/disabled") != 1 || countCalls(*calls, "/disable") != 1 {
		t.Fatalf("expected primary and fallback status calls, got %v", *calls)
	}
}

// Re-sending the password on every update is what broke unrelated changes
// on external users (403 "Cannot change password for external user").
// UpdateUser must never touch the password endpoint, even when the struct
// carries one.
func TestUpdateUser_NeverCallsPasswordEndpoint(t *testing.T) {
	c, calls, _ := userServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := c.UpdateUser("alice", &User{Username: "alice", Roles: []string{"Admin", "Reader"}, Password: "unchanged"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := countCalls(*calls, "/password"); n != 0 {
		t.Fatalf("expected no password call, got %d: %v", n, *calls)
	}
	if n := countCalls(*calls, "/api/users/0123456789abcdef01234567"); n != 1 {
		t.Fatalf("expected one profile PUT by ObjectId, got %v", *calls)
	}
}

func TestSetUserPassword_PutsExactlyOnceByID(t *testing.T) {
	c, calls, gotBody := userServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.SetUserPassword("alice", "n3w"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "PUT /api/users/0123456789abcdef01234567/password"
	if n := countCalls(*calls, "/password"); n != 1 || (*calls)[len(*calls)-1] != want {
		t.Fatalf("expected exactly one %q, got %v", want, *calls)
	}
	if *gotBody != `{"password":"n3w"}` {
		t.Fatalf("unexpected body %s", *gotBody)
	}
}
