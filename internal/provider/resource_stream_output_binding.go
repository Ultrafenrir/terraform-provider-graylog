package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type streamOutputBindingResource struct{ client *client.Client }

type streamOutputBindingModel struct {
	ID       types.String `tfsdk:"id"`
	StreamID types.String `tfsdk:"stream_id"`
	OutputID types.String `tfsdk:"output_id"`
}

func NewStreamOutputBindingResource() resource.Resource { return &streamOutputBindingResource{} }

func (r *streamOutputBindingResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_stream_output_binding"
}

func (r *streamOutputBindingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages attachment (binding) of an Output to a Stream.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, Description: "Synthetic ID: <stream_id>/<output_id>"},
			"stream_id": schema.StringAttribute{Required: true, Description: "Stream ID"},
			"output_id": schema.StringAttribute{Required: true, Description: "Output ID"},
		},
	}
}

func (r *streamOutputBindingResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *streamOutputBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data streamOutputBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sid := data.StreamID.ValueString()
	oid := data.OutputID.ValueString()
	// Diff-aware Create: attach only if not already bound
	already := false
	if sid != "" {
		if outs, err := r.client.WithContext(ctx).ListStreamOutputs(sid); err == nil {
			for _, o := range outs {
				if o.ID == oid {
					already = true
					break
				}
			}
		} else {
			resp.Diagnostics.AddError("Error listing stream outputs before attach", err.Error())
			return
		}
	}
	if !already {
		if err := r.client.WithContext(ctx).AttachOutputToStream(sid, oid); err != nil {
			resp.Diagnostics.AddError("Error attaching output to stream", err.Error())
			return
		}
	}
	data.ID = types.StringValue(syntheticBindingID(data.StreamID.ValueString(), data.OutputID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamOutputBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data streamOutputBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// If state is missing IDs but has composite id, parse it
	if (data.StreamID.IsNull() || data.StreamID.IsUnknown() || data.StreamID.ValueString() == "") &&
		(data.OutputID.IsNull() || data.OutputID.IsUnknown() || data.OutputID.ValueString() == "") &&
		!data.ID.IsNull() && !data.ID.IsUnknown() {
		sID, oID := parseBindingID(data.ID.ValueString())
		if sID != "" && oID != "" {
			data.StreamID = types.StringValue(sID)
			data.OutputID = types.StringValue(oID)
		}
	}
	outs, err := r.client.WithContext(ctx).ListStreamOutputs(data.StreamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing stream outputs", err.Error())
		return
	}
	found := false
	for _, o := range outs {
		if o.ID == data.OutputID.ValueString() {
			found = true
			break
		}
	}
	if !found {
		// binding no longer exists
		resp.State.RemoveResource(ctx)
		return
	}
	// Keep synthetic ID stable
	data.ID = types.StringValue(syntheticBindingID(data.StreamID.ValueString(), data.OutputID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamOutputBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state streamOutputBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Compute minimal changes
	oldSID, oldOID := state.StreamID.ValueString(), state.OutputID.ValueString()
	newSID, newOID := plan.StreamID.ValueString(), plan.OutputID.ValueString()
	if oldSID == newSID && oldOID == newOID {
		// nothing to do
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// 1) Ensure new binding exists (attach only if missing)
	if newSID != "" && newOID != "" {
		needAttach := true
		outs, err := r.client.WithContext(ctx).ListStreamOutputs(newSID)
		if err != nil {
			resp.Diagnostics.AddError("Error listing outputs for new stream", err.Error())
			return
		}
		for _, o := range outs {
			if o.ID == newOID {
				needAttach = false
				break
			}
		}
		if needAttach {
			if err := r.client.WithContext(ctx).AttachOutputToStream(newSID, newOID); err != nil {
				resp.Diagnostics.AddError("Error attaching new binding", err.Error())
				return
			}
		}
	}

	// 2) Remove old binding if it still exists and differs from new
	if (oldSID != "" && oldOID != "") && (oldSID != newSID || oldOID != newOID) {
		outs, err := r.client.WithContext(ctx).ListStreamOutputs(oldSID)
		if err != nil {
			resp.Diagnostics.AddError("Error listing outputs for old stream", err.Error())
			return
		}
		shouldDetach := false
		for _, o := range outs {
			if o.ID == oldOID {
				shouldDetach = true
				break
			}
		}
		if shouldDetach {
			if err := r.client.WithContext(ctx).DetachOutputFromStream(oldSID, oldOID); err != nil {
				resp.Diagnostics.AddError("Error detaching previous binding", err.Error())
				return
			}
		}
	}

	plan.ID = types.StringValue(syntheticBindingID(newSID, newOID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *streamOutputBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data streamOutputBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sid := data.StreamID.ValueString()
	oid := data.OutputID.ValueString()
	if sid == "" || oid == "" {
		// Try to parse from synthetic ID
		sid, oid = parseBindingID(data.ID.ValueString())
	}
	if sid != "" && oid != "" {
		// Detach only if present
		outs, err := r.client.WithContext(ctx).ListStreamOutputs(sid)
		if err != nil {
			resp.Diagnostics.AddError("Error listing stream outputs before detach", err.Error())
			return
		}
		present := false
		for _, o := range outs {
			if o.ID == oid {
				present = true
				break
			}
		}
		if present {
			if err := r.client.WithContext(ctx).DetachOutputFromStream(sid, oid); err != nil {
				resp.Diagnostics.AddError("Error detaching output from stream", err.Error())
				return
			}
		}
	}
}

func (r *streamOutputBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	raw := strings.TrimSpace(req.ID)
	if raw == "" {
		resp.Diagnostics.AddError("Empty import ID", "Provide 'stream_id/output_id' or 'stream_id:output_id'.")
		return
	}
	sid, oid := parseBindingID(raw)
	if sid == "" || oid == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected 'stream_id/output_id' or 'stream_id:output_id'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), syntheticBindingID(sid, oid))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("stream_id"), sid)...) //nolint:errcheck
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("output_id"), oid)...) //nolint:errcheck
}

func syntheticBindingID(streamID, outputID string) string {
	return fmt.Sprintf("%s/%s", streamID, outputID)
}

func parseBindingID(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	// support both '/' and ':' as separators
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return "", ""
}
