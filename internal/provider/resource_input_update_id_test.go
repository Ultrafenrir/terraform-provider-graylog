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

// Regression test for a critical bug found via live testing against a real Graylog 6.0.14
// server: Update() read `id` only from the plan. Since `id` is Computed (not Optional) with no
// UseStateForUnknown plan modifier, it is Unknown/empty in the plan during Update — every real
// update request went to "PUT /api/system/inputs/" (no ID) and Graylog rejected it with 405.
func TestInputResource_Update_UsesStateIDNotPlanID(t *testing.T) {
	ctx := context.Background()
	r := &inputResource{}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics)
	}

	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPut:
			gotPath = req.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "existing-id-123"})
		case req.Method == http.MethodGet && req.URL.Path == "/api/system/inputs/existing-id-123/extractors":
			_ = json.NewEncoder(w).Encode(map[string]any{"extractors": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := client.New(ts.URL, "dGVzdA==")
	c.MaxRetries = 0
	r.client = c

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	// Note: "id" is deliberately left null in the plan, exactly like a real Computed-only
	// attribute with no plan modifier — the plan never carries it, only prior state does.
	planVal := buildFullyNullValue(schemaType, map[string]tftypes.Value{
		"title": tftypes.NewValue(tftypes.String, "my-kafka-input"),
		"type":  tftypes.NewValue(tftypes.String, "org.graylog2.inputs.raw.kafka.RawKafkaInput"),
	})
	stateVal := buildFullyNullValue(schemaType, map[string]tftypes.Value{
		"id":    tftypes.NewValue(tftypes.String, "existing-id-123"),
		"title": tftypes.NewValue(tftypes.String, "my-kafka-input"),
		"type":  tftypes.NewValue(tftypes.String, "org.graylog2.inputs.raw.kafka.RawKafkaInput"),
	})

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: planVal, Schema: schemaResp.Schema},
		State: tfsdk.State{Raw: stateVal, Schema: schemaResp.Schema},
	}
	resp := resource.UpdateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	r.Update(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}
	if gotPath != "/api/system/inputs/existing-id-123" {
		t.Fatalf("expected UpdateInput to PUT to '/api/system/inputs/existing-id-123', got %q", gotPath)
	}

	var got inputModel
	diags := resp.State.Get(ctx, &got)
	if diags.HasError() {
		t.Fatalf("failed to read persisted state: %v", diags)
	}
	if got.ID.ValueString() != "existing-id-123" {
		t.Fatalf("expected persisted ID 'existing-id-123', got %q", got.ID.ValueString())
	}
}
