package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLiveSuitesContainNoHTTPMocks protects the meaning of the integration and
// acceptance build tags. Fast unit tests may use httptest, but these suites must
// always exercise the configured Graylog/OpenSearch services.
func TestLiveSuitesContainNoHTTPMocks(t *testing.T) {
	patterns := []string{"integration_*_test.go", "*_acc_test.go"}
	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %q: %v", pattern, err)
		}
		for _, file := range files {
			contents, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			text := string(contents)
			for _, forbidden := range []string{"net/http/httptest", "httptest.NewServer", "RoundTripper"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("%s contains %q; live suites must not mock HTTP services", file, forbidden)
				}
			}
		}
	}
}
