//go:build integration

package provider

import (
	"encoding/base64"
	"errors"
	"os"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func clusterConfigTestClient(t *testing.T) *client.Client {
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

// State keeps the practitioner's document, and Read compares only the keys
// in it, so a class that materializes extra keys into the stored document
// never causes a diff. This asserts the stronger property for the probe
// classes it chooses: those are stored verbatim on every matrix version, and
// writing the echo back unchanged is a no-op. It is deliberately not a claim
// about every class: org.graylog.plugins.map.config.GeoIpResolverConfig is a
// known non-verbatim one, echoing eight keys that were never sent, one of
// them the EncryptedValue sentinel {"is_set": false} that Graylog itself
// rejects on a write.
func TestIntegration_ClusterConfigRoundTripIsVerbatim(t *testing.T) {
	c := clusterConfigTestClient(t)

	var class string
	var original []byte
	for _, candidate := range clusterConfigProbeClasses {
		doc, err := c.GetClusterConfig(candidate)
		if err != nil {
			continue
		}
		class, original = candidate, doc
		break
	}
	if class == "" {
		t.Skipf("none of the probe classes returned a document: %v", clusterConfigProbeClasses)
	}
	t.Logf("using class %s", class)

	canonicalOriginal, err := CanonicalizeJSONFromString(string(original))
	if err != nil {
		t.Fatalf("server document is not valid JSON: %v", err)
	}

	// Writing the document back unchanged must be a no-op, byte-for-byte
	// after canonicalization.
	if _, err := c.UpdateClusterConfig(class, original); err != nil {
		t.Fatalf("UpdateClusterConfig error: %v", err)
	}

	readBack, err := c.GetClusterConfig(class)
	if err != nil {
		t.Fatalf("GetClusterConfig after write error: %v", err)
	}
	canonicalReadBack, err := CanonicalizeJSONFromString(string(readBack))
	if err != nil {
		t.Fatalf("document read back is not valid JSON: %v", err)
	}

	if canonicalReadBack != canonicalOriginal {
		t.Fatalf("cluster configuration was not stored verbatim for %s\n before: %s\n  after: %s",
			class, canonicalOriginal, canonicalReadBack)
	}
}

// Graylog answers 204 for a known class with nothing stored and 404 for a
// class it cannot resolve. The client must fold both into ErrNotFound so the
// resource drops out of state rather than erroring.
func TestIntegration_ClusterConfigUnknownClassIsNotFound(t *testing.T) {
	c := clusterConfigTestClient(t)

	// Inside the default safe_classes prefixes, so this reaches class
	// resolution instead of being rejected by the allowlist.
	_, err := c.GetClusterConfig(clusterConfigAbsentClass)
	if !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for an unresolvable class, got %v", err)
	}
}

// Destroy must converge even when the document is already gone, otherwise a
// partially applied destroy can never be completed.
func TestIntegration_ClusterConfigDeleteAbsentIsIdempotent(t *testing.T) {
	c := clusterConfigTestClient(t)

	if err := c.DeleteClusterConfig(clusterConfigAbsentClass); err != nil {
		t.Fatalf("deleting an absent document should succeed, got %v", err)
	}
}

// A document that omits fields the target class requires must fail loudly at
// apply time rather than being silently half-applied.
func TestIntegration_ClusterConfigRejectsIncompleteDocument(t *testing.T) {
	c := clusterConfigTestClient(t)

	var class string
	for _, candidate := range clusterConfigProbeClasses {
		if _, err := c.GetClusterConfig(candidate); err == nil {
			class = candidate
			break
		}
	}
	if class == "" {
		t.Skipf("none of the probe classes returned a document: %v", clusterConfigProbeClasses)
	}

	if _, err := c.UpdateClusterConfig(class, []byte(`{"tf_provider_not_a_real_field":true}`)); err == nil {
		t.Fatalf("expected %s to reject a document missing its required fields", class)
	}
}
