package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// buildFullyNullValue constructs a tftypes.Value for the given object type where every
// attribute is null except the ones supplied in overrides. Useful for building minimal
// synthetic Plan/State values in tests without hand-writing every nested attribute type.
func buildFullyNullValue(typ tftypes.Type, overrides map[string]tftypes.Value) tftypes.Value {
	obj, ok := typ.(tftypes.Object)
	if !ok {
		return tftypes.NewValue(typ, nil)
	}
	values := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, t := range obj.AttributeTypes {
		if v, ok := overrides[name]; ok {
			values[name] = v
			continue
		}
		values[name] = tftypes.NewValue(t, nil)
	}
	return tftypes.NewValue(obj, values)
}

// newTestIndexSetClient builds a *client.Client against an httptest server without letting
// client.New's version-detection probe (a GET to /api/system and /system) interfere; the fake
// server just answers those with 404 like a server with no recognizable version header would.
func newTestIndexSetClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	c := client.New(baseURL, "dGVzdA==")
	// Disable retry backoff so a simulated server error fails fast instead of taking seconds.
	c.MaxRetries = 0
	return c
}

func TestIndexSetResource_Create_PersistsIDOnPostCreateReadFailure(t *testing.T) {
	ctx := context.Background()
	r := &indexSetResource{}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/system/indices/index_sets":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "created-id-123"})
		case req.Method == http.MethodGet && req.URL.Path == "/api/system/indices/index_sets/created-id-123":
			// Simulate a transient failure reading back the just-created index set.
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	r.client = newTestIndexSetClient(t, ts.URL)

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	planVal := buildFullyNullValue(schemaType, map[string]tftypes.Value{
		"title":        tftypes.NewValue(tftypes.String, "my-index"),
		"index_prefix": tftypes.NewValue(tftypes.String, "myidx"),
	})

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Raw: planVal, Schema: schemaResp.Schema},
	}
	resp := resource.CreateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.Create(ctx, req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error diagnostic from the failed post-create read")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("expected state to be persisted despite the read failure, to avoid orphaning the created index set")
	}

	var got indexSetModel
	diags := resp.State.Get(ctx, &got)
	if diags.HasError() {
		t.Fatalf("failed to read persisted state: %v", diags)
	}
	if got.ID.ValueString() != "created-id-123" {
		t.Fatalf("expected persisted ID 'created-id-123', got %q", got.ID.ValueString())
	}
}

func TestIndexSetResource_Update_PersistsStateOnPostUpdateReadFailure(t *testing.T) {
	ctx := context.Background()
	r := &indexSetResource{}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics)
	}

	getCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/system/indices/index_sets/idx-1":
			getCount++
			if getCount == 1 {
				// First GET: UpdateIndexSet's own read-modify-write step. Must succeed so the
				// subsequent PUT can go through.
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "idx-1", "title": "old-title", "index_prefix": "myidx",
					"shards": 1, "replicas": 0, "index_analyzer": "standard",
				})
				return
			}
			// Second GET: the resource's post-update read-back — simulate it failing.
			w.WriteHeader(http.StatusInternalServerError)
		case req.Method == http.MethodPut && req.URL.Path == "/api/system/indices/index_sets/idx-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "idx-1"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	r.client = newTestIndexSetClient(t, ts.URL)

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	planVal := buildFullyNullValue(schemaType, map[string]tftypes.Value{
		"title":        tftypes.NewValue(tftypes.String, "updated-title"),
		"index_prefix": tftypes.NewValue(tftypes.String, "myidx"),
	})
	stateVal := buildFullyNullValue(schemaType, map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "idx-1"),
		"title":        tftypes.NewValue(tftypes.String, "old-title"),
		"index_prefix": tftypes.NewValue(tftypes.String, "myidx"),
	})

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: planVal, Schema: schemaResp.Schema},
		State: tfsdk.State{Raw: stateVal, Schema: schemaResp.Schema},
	}
	resp := resource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.Update(ctx, req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error diagnostic from the failed post-update read")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("expected state to still be persisted despite the read failure")
	}

	var got indexSetModel
	diags := resp.State.Get(ctx, &got)
	if diags.HasError() {
		t.Fatalf("failed to read persisted state: %v", diags)
	}
	if got.ID.ValueString() != "idx-1" {
		t.Fatalf("expected ID to be preserved as 'idx-1', got %q", got.ID.ValueString())
	}
}
