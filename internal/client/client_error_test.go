package client

import (
	"encoding/json"
	"testing"
)

func TestParseGraylogError_NonJSON(t *testing.T) {
	err := ParseGraylogError(500, []byte("internal server error"))
	ge, ok := err.(*GraylogError)
	if !ok {
		t.Fatalf("expected *GraylogError, got %T", err)
	}
	if ge.Status != 500 {
		t.Fatalf("status mismatch: %d", ge.Status)
	}
	if ge.Message == "" {
		t.Fatalf("expected message to be set from raw body")
	}
}

func TestParseGraylogError_MessageField(t *testing.T) {
	payload := map[string]any{"type": "ApiError", "message": "bad request"}
	b, _ := json.Marshal(payload)
	err := ParseGraylogError(400, b)
	ge := err.(*GraylogError)
	if ge.Message != "bad request" {
		t.Fatalf("unexpected message: %q", ge.Message)
	}
	if ge.Type != "ApiError" {
		t.Fatalf("unexpected type: %q", ge.Type)
	}
}

func TestParseGraylogError_ErrorField(t *testing.T) {
	payload := map[string]any{"error": "not found"}
	b, _ := json.Marshal(payload)
	err := ParseGraylogError(404, b)
	ge := err.(*GraylogError)
	if ge.Err != "not found" {
		t.Fatalf("unexpected error field: %q", ge.Err)
	}
}

func TestParseGraylogError_ValidationMap(t *testing.T) {
	payload := map[string]any{
		"message": "validation failed",
		"errors": map[string]any{
			"title": []any{"may not be empty"},
			"id":    "invalid",
		},
	}
	b, _ := json.Marshal(payload)
	err := ParseGraylogError(422, b)
	ge := err.(*GraylogError)
	if len(ge.Errors["title"]) != 1 || ge.Errors["title"][0] != "may not be empty" {
		t.Fatalf("title validation not parsed: %+v", ge.Errors)
	}
	if len(ge.Errors["id"]) != 1 || ge.Errors["id"][0] != "invalid" {
		t.Fatalf("id validation not parsed: %+v", ge.Errors)
	}
}

func TestGraylogError_ErrorString(t *testing.T) {
	ge := &GraylogError{Status: 400, Message: "bad request"}
	s := ge.Error()
	if s == "" || s == "Graylog API error (status 400): " {
		t.Fatalf("unexpected error string: %q", s)
	}
}
