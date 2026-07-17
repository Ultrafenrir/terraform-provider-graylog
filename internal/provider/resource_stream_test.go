package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestStreamResource_New(t *testing.T) {
	r := NewStreamResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestStreamResource_UpgradeState(t *testing.T) {
	ctx := context.Background()
	r := &streamResource{}

	// Get schema to validate against
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics)
	}

	upgraders := r.UpgradeState(ctx)

	tests := []struct {
		name               string
		priorVersion       int64
		rawJSON            string
		expectRemove       bool
		expectMatchingType string
	}{
		{
			name:         "v2 state without remove_matches_from_default_stream or matching_type",
			priorVersion: 2,
			rawJSON: `{
				"id": "stream-123",
				"title": "Test Stream",
				"description": "test",
				"disabled": false,
				"index_set_id": "index-1",
				"rule": [],
				"timeouts": null
			}`,
			expectRemove:       false,
			expectMatchingType: "AND",
		},
		{
			name:         "v3 state with remove_matches_from_default_stream=true but no matching_type",
			priorVersion: 3,
			rawJSON: `{
				"id": "stream-456",
				"title": "Test Stream 2",
				"description": "test2",
				"disabled": false,
				"index_set_id": "index-2",
				"remove_matches_from_default_stream": true,
				"rule": [],
				"timeouts": null
			}`,
			expectRemove:       true,
			expectMatchingType: "AND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader, ok := upgraders[tt.priorVersion]
			if !ok {
				t.Fatalf("no upgrader registered for prior version %d", tt.priorVersion)
			}

			req := resource.UpgradeStateRequest{
				RawState: &tfprotov6.RawState{JSON: []byte(tt.rawJSON)},
			}
			resp := resource.UpgradeStateResponse{
				State: tfsdk.State{
					Schema: schemaResp.Schema,
				},
			}

			upgrader.StateUpgrader(ctx, req, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}

			// Verify upgraded state
			var upgraded streamModel
			diags := resp.State.Get(ctx, &upgraded)
			if diags.HasError() {
				t.Fatalf("failed to get upgraded state: %v", diags)
			}

			if upgraded.RemoveMatchesFromDefault.IsNull() {
				t.Error("remove_matches_from_default_stream should not be null after upgrade")
			}
			if upgraded.RemoveMatchesFromDefault.ValueBool() != tt.expectRemove {
				t.Errorf("expected remove_matches_from_default_stream=%v, got %v",
					tt.expectRemove, upgraded.RemoveMatchesFromDefault.ValueBool())
			}

			if upgraded.MatchingType.IsNull() {
				t.Error("matching_type should not be null after upgrade")
			}
			if upgraded.MatchingType.ValueString() != tt.expectMatchingType {
				t.Errorf("expected matching_type=%v, got %v",
					tt.expectMatchingType, upgraded.MatchingType.ValueString())
			}
		})
	}
}
