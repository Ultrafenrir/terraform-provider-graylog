package provider

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type inputResource struct{ client *client.Client }

type inputModel struct {
	ID            types.String   `tfsdk:"id"`
	Title         types.String   `tfsdk:"title"`
	Type          types.String   `tfsdk:"type"`
	Global        types.Bool     `tfsdk:"global"`
	Node          types.String   `tfsdk:"node"`
	Configuration types.String   `tfsdk:"configuration"`
	Extractors    types.String   `tfsdk:"extractors"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewInputResource() resource.Resource { return &inputResource{} }

func (r *inputResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_input"
}

func (r *inputResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     6,
		Description: "Manages a Graylog input resource. Compatible with Graylog v5, v6, and v7.\n- Supports all Kafka input types and settings via flexible configuration.\n- Supports managing input extractors.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, Description: "The unique identifier of the input"},
			"title": schema.StringAttribute{Required: true, Description: "The title of the input"},
			"type":  schema.StringAttribute{Required: true, Description: "The input type (e.g., org.graylog2.inputs.syslog.udp.SyslogUDPInput)"},
			"global": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether this input is global (available on all nodes)",
			},
			"node": schema.StringAttribute{
				Optional:    true,
				Description: "Node ID to run this input on (if not global)",
			},
			// Use JSON-encoded strings to represent free-form objects to satisfy framework limitations.
			"configuration": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded object with input configuration (free-form).",
			},
			"extractors": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded list of extractor objects (free-form).",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

// validateInput performs runtime validation of the input model and appends diagnostics on issues.
func validateInput(ctx context.Context, data *inputModel) (diags diag.Diagnostics) {
	if data.Title.IsNull() || data.Title.IsUnknown() || data.Title.ValueString() == "" {
		diags.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if data.Type.IsNull() || data.Type.IsUnknown() || data.Type.ValueString() == "" {
		diags.AddAttributeError(path.Root("type"), "Invalid type", "Attribute 'type' must be a non-empty string with a Graylog input class name.")
	}
	// Cross-field: global/node
	if !data.Global.IsUnknown() && !data.Global.IsNull() && !data.Global.ValueBool() {
		if data.Node.IsNull() || data.Node.IsUnknown() || data.Node.ValueString() == "" {
			diags.AddAttributeError(path.Root("node"), "Missing node for non-global input", "When 'global' is false, 'node' must be specified with a non-empty node ID.")
		}
	}
	// Extractors: optional JSON string
	return
}

func (r *inputResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *inputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateInput(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Convert configuration (JSON string) into map[string]interface{}
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() && data.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &config); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid configuration JSON", err.Error())
			return
		}
	}

	in := &client.Input{
		Title:         data.Title.ValueString(),
		Type:          data.Type.ValueString(),
		Global:        data.Global.ValueBool(),
		Node:          data.Node.ValueString(),
		Configuration: config,
	}
	created, err := r.client.CreateInput(in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating input", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	// Create extractors if provided (JSON list of maps)
	if !data.Extractors.IsNull() && !data.Extractors.IsUnknown() && data.Extractors.ValueString() != "" {
		var extractorObjs []map[string]interface{}
		if err := json.Unmarshal([]byte(data.Extractors.ValueString()), &extractorObjs); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("extractors"), "Invalid extractors JSON", err.Error())
			return
		}
		for _, ex := range extractorObjs {
			// Support either top-level extractor map or nested under "data"
			payload, ok := ex["data"].(map[string]interface{})
			if !ok || payload == nil {
				payload = ex
			}
			if _, err := r.client.CreateInputExtractor(created.ID, payload); err != nil {
				resp.Diagnostics.AddError("Error creating input extractor", err.Error())
				return
			}
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, err := r.client.GetInput(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Resource was deleted outside of Terraform
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading input", err.Error())
		return
	}
	data.Title = types.StringValue(in.Title)
	data.Type = types.StringValue(in.Type)
	data.Global = types.BoolValue(in.Global)
	data.Node = types.StringValue(in.Node)

	// Set configuration back as JSON string
	if in.Configuration != nil {
		if b, err := json.Marshal(in.Configuration); err == nil {
			data.Configuration = types.StringValue(string(b))
		}
	}

	// Read extractors
	if exList, err := r.client.ListInputExtractors(data.ID.ValueString()); err == nil {
		if len(exList) == 0 {
			data.Extractors = types.StringNull()
		} else if b, err2 := json.Marshal(exList); err2 == nil {
			data.Extractors = types.StringValue(string(b))
		}
	} else {
		resp.Diagnostics.AddWarning("Unable to read input extractors", err.Error())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateInput(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Convert configuration JSON to map
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() && data.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &config); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid configuration JSON", err.Error())
			return
		}
	}

	in := &client.Input{
		Title:         data.Title.ValueString(),
		Type:          data.Type.ValueString(),
		Global:        data.Global.ValueBool(),
		Node:          data.Node.ValueString(),
		Configuration: config,
	}
	_, err := r.client.UpdateInput(data.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating input", err.Error())
		return
	}
	// Reconcile extractors: delete all existing and recreate from desired list
	// (simple strategy to keep provider logic maintainable)
	if exExisting, err := r.client.ListInputExtractors(data.ID.ValueString()); err == nil {
		for _, ex := range exExisting {
			if id, ok := ex["id"].(string); ok && id != "" {
				if derr := r.client.DeleteInputExtractor(data.ID.ValueString(), id); derr != nil {
					resp.Diagnostics.AddWarning("Failed to delete existing extractor", derr.Error())
				}
			}
		}
	}
	if !data.Extractors.IsNull() && !data.Extractors.IsUnknown() && data.Extractors.ValueString() != "" {
		var extractorObjs []map[string]interface{}
		if err := json.Unmarshal([]byte(data.Extractors.ValueString()), &extractorObjs); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("extractors"), "Invalid extractors JSON", err.Error())
			return
		}
		for _, ex := range extractorObjs {
			payload, ok := ex["data"].(map[string]interface{})
			if !ok || payload == nil {
				payload = ex
			}
			if _, cerr := r.client.CreateInputExtractor(data.ID.ValueString(), payload); cerr != nil {
				resp.Diagnostics.AddError("Error creating input extractor", cerr.Error())
				return
			}
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	deleteTimeout, diags := data.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	if err := r.client.DeleteInput(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting input", err.Error())
	}
}

func (r *inputResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
