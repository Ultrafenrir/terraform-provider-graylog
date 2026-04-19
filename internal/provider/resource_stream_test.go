package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

	tests := []struct {
		name          string
		priorVersion  int64
		priorState    map[string]tftypes.Value
		expectRemove  bool
	}{
		{
			name:         "v2 state without remove_matches_from_default_stream",
			priorVersion: 2,
			priorState: map[string]tftypes.Value{
				"id":          tftypes.NewValue(tftypes.String, "stream-123"),
				"title":       tftypes.NewValue(tftypes.String, "Test Stream"),
				"description": tftypes.NewValue(tftypes.String, "test"),
				"disabled":    tftypes.NewValue(tftypes.Bool, false),
				"index_set_id": tftypes.NewValue(tftypes.String, "index-1"),
				"remove_matches_from_default_stream": tftypes.NewValue(tftypes.Bool, nil), // null value
				"rule":        tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"id":          tftypes.String,
						"field":       tftypes.String,
						"type":        tftypes.Number,
						"value":       tftypes.String,
						"inverted":    tftypes.Bool,
						"description": tftypes.String,
					},
				}}, []tftypes.Value{}),
				"timeouts": tftypes.NewValue(tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"create": tftypes.String,
						"update": tftypes.String,
						"delete": tftypes.String,
					},
				}, nil),
			},
			expectRemove: false,
		},
		{
			name:         "v2 state with remove_matches_from_default_stream=true",
			priorVersion: 2,
			priorState: map[string]tftypes.Value{
				"id":          tftypes.NewValue(tftypes.String, "stream-456"),
				"title":       tftypes.NewValue(tftypes.String, "Test Stream 2"),
				"description": tftypes.NewValue(tftypes.String, "test2"),
				"disabled":    tftypes.NewValue(tftypes.Bool, false),
				"index_set_id": tftypes.NewValue(tftypes.String, "index-2"),
				"remove_matches_from_default_stream": tftypes.NewValue(tftypes.Bool, true),
				"rule": tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"id":          tftypes.String,
						"field":       tftypes.String,
						"type":        tftypes.Number,
						"value":       tftypes.String,
						"inverted":    tftypes.Bool,
						"description": tftypes.String,
					},
				}}, []tftypes.Value{}),
				"timeouts": tftypes.NewValue(tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"create": tftypes.String,
						"update": tftypes.String,
						"delete": tftypes.String,
					},
				}, nil),
			},
			expectRemove: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create prior state
			priorStateVal := tftypes.NewValue(
				tftypes.Object{AttributeTypes: map[string]tftypes.Type{
					"id":          tftypes.String,
					"title":       tftypes.String,
					"description": tftypes.String,
					"disabled":    tftypes.Bool,
					"index_set_id": tftypes.String,
					"remove_matches_from_default_stream": tftypes.Bool,
					"rule": tftypes.List{ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"id":          tftypes.String,
							"field":       tftypes.String,
							"type":        tftypes.Number,
							"value":       tftypes.String,
							"inverted":    tftypes.Bool,
							"description": tftypes.String,
						},
					}},
					"timeouts": tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"create": tftypes.String,
							"update": tftypes.String,
							"delete": tftypes.String,
						},
					},
				}},
				tt.priorState,
			)

			req := resource.UpgradeStateRequest{
				State: &tfsdk.State{
					Raw:    priorStateVal,
					Schema: schemaResp.Schema,
				},
			}

			resp := resource.UpgradeStateResponse{
				State: tfsdk.State{
					Schema: schemaResp.Schema,
				},
			}

			r.UpgradeState(ctx, req, &resp)

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
		})
	}
}
