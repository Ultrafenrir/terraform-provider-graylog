package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// The document Graylog 7.1.5 returns for
// org.graylog.plugins.map.config.GeoIpResolverConfig after geoIPResolverConfig
// was written: eight keys the practitioner never sent, one of them the
// EncryptedValue sentinel that the server itself refuses on a write
// ("set_value must be a string and cannot be missing").
const geoIPResolverEcho = `{"azure_cloud":false,"enabled":true,"enforce_graylog_schema":true,"db_vendor_type":"MAXMIND","city_db_path":"/usr/share/graylog/data/geolocation/GeoLite2-City.mmdb","asn_db_path":"","use_s3":false,"pull_from_cloud":null,"gcs_project_id":null,"refresh_interval_unit":"MINUTES","refresh_interval":10,"azure_account":null,"azure_account_key":{"is_set":false},"azure_container":null,"azure_endpoint":null}`

const geoIPResolverConfig = `{"enabled":true,"enforce_graylog_schema":true,"db_vendor_type":"MAXMIND","city_db_path":"/usr/share/graylog/data/geolocation/GeoLite2-City.mmdb","asn_db_path":"","refresh_interval_unit":"MINUTES","refresh_interval":10}`

// An unchanged configuration must refresh to itself; otherwise every plan
// proposes removing the keys the server materialized on its own.
func TestRefreshClusterConfigDocument_ProjectsOntoManagedKeys(t *testing.T) {
	got, err := refreshClusterConfigDocument(geoIPResolverEcho, geoIPResolverConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, err := CanonicalizeJSONFromString(geoIPResolverConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("unchanged configuration would show a diff\n want: %s\n got : %s", want, got)
	}
}

func TestRefreshClusterConfigDocument_ReportsRealDrift(t *testing.T) {
	server := strings.Replace(geoIPResolverEcho, `"enabled":true`, `"enabled":false`, 1)

	got, err := refreshClusterConfigDocument(server, geoIPResolverConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, err := CanonicalizeJSONFromString(geoIPResolverConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == want {
		t.Fatal("a server-side change to a managed key went unnoticed")
	}
	if !strings.Contains(got, `"enabled":false`) {
		t.Fatalf("server value did not win: %s", got)
	}
}

// An imported resource has no document to project against, so the whole
// server document is adopted, minus the sentinel that could never be written
// back. The adopted document then becomes the mask for the next read and
// must refresh to itself.
func TestRefreshClusterConfigDocument_ImportAdoptsServerDocumentWithoutSentinels(t *testing.T) {
	got, err := refreshClusterConfigDocument(geoIPResolverEcho, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	for _, key := range []string{"azure_cloud", "use_s3", "pull_from_cloud", "gcs_project_id", "azure_account", "azure_container", "azure_endpoint"} {
		if _, present := doc[key]; !present {
			t.Fatalf("import should adopt the whole document, %s is missing: %s", key, got)
		}
	}
	if _, present := doc["azure_account_key"]; present || strings.Contains(got, "is_set") {
		t.Fatalf("the unwritable sentinel reached state: %s", got)
	}

	again, err := refreshClusterConfigDocument(geoIPResolverEcho, got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if again != got {
		t.Fatalf("imported document does not converge\n first : %s\n second: %s", got, again)
	}
}

// Only the exact {"is_set": <bool>} shape is a sentinel; anything else the
// practitioner may have written stays, and sentinels nested deeper go too.
func TestRefreshClusterConfigDocument_SentinelShape(t *testing.T) {
	server := `{"secret":{"is_set":true},"nested":{"inner":{"is_set":false},"kept":1},"flag":{"is_set":"yes"},"pair":{"is_set":true,"other":1}}`
	want := `{"flag":{"is_set":"yes"},"nested":{"kept":1},"pair":{"is_set":true,"other":1}}`

	got, err := refreshClusterConfigDocument(server, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("sentinel stripping mismatch\n want: %s\n got : %s", want, got)
	}
}

func TestRefreshClusterConfigDocument_InvalidServerDocument(t *testing.T) {
	if _, err := refreshClusterConfigDocument(`{not json`, `{}`); err == nil {
		t.Fatal("expected an error for an unparseable server document")
	}
}
