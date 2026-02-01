package provider

import (
	"context"
	"encoding/json"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type outputResource struct{ client *client.Client }

type outputModel struct {
	ID            types.String   `tfsdk:"id"`
	Title         types.String   `tfsdk:"title"`
	Type          types.String   `tfsdk:"type"`
	Configuration types.String   `tfsdk:"configuration"`
	Streams       []types.String `tfsdk:"streams"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewOutputResource() resource.Resource { return &outputResource{} }

func (r *outputResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_output"
}

func (r *outputResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Graylog Output and its attachments to streams.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Output ID"},
			"title":         schema.StringAttribute{Required: true, Description: "Output title"},
			"type":          schema.StringAttribute{Required: true, Description: "Output type (FQCN)"},
			"configuration": schema.StringAttribute{Optional: true, Description: "JSON-encoded configuration object"},
			"streams":       schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Stream IDs to attach this output to"},
			"timeouts":      timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *outputResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *outputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data outputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg := map[string]interface{}{}
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() && data.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid JSON", err.Error())
			return
		}
	}
	created, err := r.client.WithContext(ctx).CreateOutput(&client.Output{
		Title:         data.Title.ValueString(),
		Type:          data.Type.ValueString(),
		Configuration: cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating output", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	// Attach to streams if provided
	for _, s := range data.Streams {
		if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
			continue
		}
		if err := r.client.WithContext(ctx).AttachOutputToStream(s.ValueString(), data.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error attaching output to stream", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *outputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data outputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.WithContext(ctx).GetOutput(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading output", err.Error())
		return
	}
	data.Title = types.StringValue(o.Title)
	data.Type = types.StringValue(o.Type)
	if b, err := json.Marshal(o.Configuration); err == nil {
		data.Configuration = types.StringValue(string(b))
	}
	// streams: оставляем как в состоянии (нет простого способа прочитать привязки для данного output)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *outputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan outputModel
	var state outputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)   // new
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...) // old
	if resp.Diagnostics.HasError() {
		return
	}
	cfg := map[string]interface{}{}
	if !plan.Configuration.IsNull() && !plan.Configuration.IsUnknown() && plan.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(plan.Configuration.ValueString()), &cfg); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid JSON", err.Error())
			return
		}
	}
	if _, err := r.client.WithContext(ctx).UpdateOutput(plan.ID.ValueString(), &client.Output{
		Title:         plan.Title.ValueString(),
		Type:          plan.Type.ValueString(),
		Configuration: cfg,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating output", err.Error())
		return
	}
	// sync attachments by delta between state.Streams and plan.Streams
	oldSet := map[string]struct{}{}
	for _, s := range state.Streams {
		if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
			continue
		}
		oldSet[s.ValueString()] = struct{}{}
	}
	newSet := map[string]struct{}{}
	for _, s := range plan.Streams {
		if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
			continue
		}
		newSet[s.ValueString()] = struct{}{}
	}
	// detach
	for id := range oldSet {
		if _, ok := newSet[id]; !ok {
			if err := r.client.WithContext(ctx).DetachOutputFromStream(id, plan.ID.ValueString()); err != nil {
				resp.Diagnostics.AddError("Error detaching output from stream", err.Error())
				return
			}
		}
	}
	// attach
	for id := range newSet {
		if _, ok := oldSet[id]; !ok {
			if err := r.client.WithContext(ctx).AttachOutputToStream(id, plan.ID.ValueString()); err != nil {
				resp.Diagnostics.AddError("Error attaching output to stream", err.Error())
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *outputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data outputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.WithContext(ctx).DeleteOutput(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting output", err.Error())
		return
	}
}

func (r *outputResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
