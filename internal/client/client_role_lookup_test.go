package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func roleServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/system" || r.URL.Path == "/system" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"7.1.0"}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(ts.Close)
	c := New(ts.URL, "dGVzdA==")
	c.MaxRetries = 0
	return c
}

// The endpoint's own search is a substring match: asking for "Reader" also
// returns "API Browser Reader". Trusting the first hit would silently bind
// the wrong role.
func TestGetRoleByName_MatchesExactlyNotSubstring(t *testing.T) {
	c := roleServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":2,"page":1,"per_page":100,"count":2,"roles":[
			{"id":"wrong","name":"API Browser Reader","description":"","permissions":[],"read_only":true},
			{"id":"right","name":"Reader","description":"d","permissions":["streams:read"],"read_only":true}
		]}`))
	})

	role, err := c.GetRoleByName("Reader")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.ID != "right" {
		t.Fatalf("matched the wrong role: %+v", role)
	}
	if role.Name != "Reader" || len(role.Permissions) != 1 || !role.ReadOnly {
		t.Fatalf("role fields not decoded: %+v", role)
	}
}

// A role beyond the first page must still be found.
func TestGetRoleByName_Paginates(t *testing.T) {
	var pages int
	c := roleServer(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		page := r.URL.Query().Get("page")
		w.WriteHeader(http.StatusOK)
		if page == "1" {
			roles := ""
			for i := 0; i < 100; i++ {
				if i > 0 {
					roles += ","
				}
				roles += fmt.Sprintf(`{"id":"f%d","name":"filler%d","permissions":[]}`, i, i)
			}
			_, _ = w.Write([]byte(`{"total":101,"page":1,"per_page":100,"count":100,"roles":[` + roles + `]}`))
			return
		}
		_, _ = w.Write([]byte(`{"total":101,"page":2,"per_page":100,"count":1,"roles":[{"id":"target","name":"Wanted","permissions":[]}]}`))
	})

	role, err := c.GetRoleByName("Wanted")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.ID != "target" {
		t.Fatalf("expected the role from page 2, got %+v", role)
	}
	if pages < 2 {
		t.Fatalf("expected pagination, only %d request(s) made", pages)
	}
}

// The server may cap per_page below what was asked for. The walk has to
// follow the size it echoes back, or a role on page 2 is reported missing.
// The name carries a space so the query round-trips through escaping.
func TestGetRoleByName_FollowsCappedPerPage(t *testing.T) {
	var pages int
	c := roleServer(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if got := r.URL.Query().Get("query"); got != "Wanted Role" {
			t.Errorf("name not sent as query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"total":3,"page":1,"per_page":2,"count":2,"roles":[
				{"id":"a","name":"Wanted Role A","permissions":[]},
				{"id":"b","name":"Wanted Role B","permissions":[]}
			]}`))
			return
		}
		_, _ = w.Write([]byte(`{"total":3,"page":2,"per_page":2,"count":1,"roles":[
			{"id":"target","name":"Wanted Role","permissions":[]}
		]}`))
	})

	role, err := c.GetRoleByName("Wanted Role")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.ID != "target" {
		t.Fatalf("expected the role from page 2, got %+v", role)
	}
	if pages != 2 {
		t.Fatalf("expected exactly 2 requests, made %d", pages)
	}
}

// Same capped server, role absent: the echoed size and total end the walk.
func TestGetRoleByName_CappedPerPageNotFound(t *testing.T) {
	var pages int
	c := roleServer(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"total":3,"page":1,"per_page":2,"count":2,"roles":[
				{"id":"a","name":"Other A","permissions":[]},
				{"id":"b","name":"Other B","permissions":[]}
			]}`))
			return
		}
		_, _ = w.Write([]byte(`{"total":3,"page":2,"per_page":2,"count":1,"roles":[
			{"id":"c","name":"Other C","permissions":[]}
		]}`))
	})

	if _, err := c.GetRoleByName("Missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if pages != 2 {
		t.Fatalf("expected exactly 2 requests, made %d", pages)
	}
}

func TestGetRoleByName_NotFound(t *testing.T) {
	c := roleServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":1,"page":1,"per_page":100,"count":1,"roles":[{"id":"x","name":"Other","permissions":[]}]}`))
	})

	if _, err := c.GetRoleByName("Missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// A server whose total overstates what it holds ends the walk with an empty
// page instead of spinning forever.
func TestGetRoleByName_EmptyPageTerminates(t *testing.T) {
	var pages int
	c := roleServer(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"total":9999,"page":1,"per_page":100,"count":1,"roles":[{"id":"x","name":"Other","permissions":[]}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"total":9999,"page":2,"per_page":100,"count":0,"roles":[]}`))
	})

	if _, err := c.GetRoleByName("Missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if pages != 2 {
		t.Fatalf("expected the walk to stop at the first empty page, made %d requests", pages)
	}
}
