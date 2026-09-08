//go:build integration

package provider

import (
	"encoding/base64"
	"errors"
	"os"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func roleTestClient(t *testing.T) *client.Client {
	t.Helper()
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	return client.New(baseURL, token)
}

// Reader is built in on every supported version, and its id is the value the
// authentication APIs actually want.
func TestIntegration_GetRoleByName(t *testing.T) {
	c := roleTestClient(t)

	role, err := c.GetRoleByName("Reader")
	if err != nil {
		t.Fatalf("GetRoleByName error: %v", err)
	}
	if role.Name != "Reader" {
		t.Fatalf("matched the wrong role: %+v", role)
	}
	if role.ID == "" {
		t.Fatal("role id is empty; the authentication APIs cannot use a name")
	}
	if role.ID == role.Name {
		t.Fatalf("id looks like a name, not an identifier: %q", role.ID)
	}
}

// Live servers ship "API Browser Reader" alongside "Reader", which is exactly
// the pair the endpoint's substring search confuses.
func TestIntegration_GetRoleByNameIsNotFooledBySubstrings(t *testing.T) {
	c := roleTestClient(t)

	reader, err := c.GetRoleByName("Reader")
	if err != nil {
		t.Fatalf("GetRoleByName error: %v", err)
	}
	other, err := c.GetRoleByName("API Browser Reader")
	if err != nil {
		t.Skipf("this version has no API Browser Reader role: %v", err)
	}
	if reader.ID == other.ID {
		t.Fatalf("two different roles resolved to the same id: %s", reader.ID)
	}
}

func TestIntegration_GetRoleByNameNotFound(t *testing.T) {
	c := roleTestClient(t)

	if _, err := c.GetRoleByName("tf-itest-no-such-role"); !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
