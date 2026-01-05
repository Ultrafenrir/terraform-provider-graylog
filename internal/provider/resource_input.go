package provider

import (
	"context"
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
	Configuration types.Map      `tfsdk:"configuration"`
	Extractors    types.List     `tfsdk:"extractors"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewInputResource() resource.Resource { return &inputResource{} }

func (r *inputResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_input"
}

func (r *inputResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     3,
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
			"configuration": schema.MapAttribute{
				ElementType: types.DynamicType,
				Optional:    true,
				Description: "Input-specific configuration parameters as key-value pairs. Values can be strings, numbers, or booleans to fully cover Kafka inputs.",
			},
			"extractors": schema.ListAttribute{
				Optional:    true,
				ElementType: types.MapType{ElemType: types.DynamicType},
				Description: "List of extractors (free-form objects) passed directly to Graylog API. Each item is a map of extractor fields.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
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

	// Apply timeout
	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Convert configuration map (allowing dynamic values for Kafka settings)
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() {
		tmp := make(map[string]interface{}, len(data.Configuration.Elements()))
		diags = data.Configuration.ElementsAs(ctx, &tmp, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range tmp {
			config[k] = v
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
	// Create extractors if provided
	if !data.Extractors.IsNull() && !data.Extractors.IsUnknown() {
		var extractorObjs []map[string]interface{}
		diags = data.Extractors.ElementsAs(ctx, &extractorObjs, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
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

	// Convert configuration to map with dynamic values
	configMap := make(map[string]interface{})
	for k, v := range in.Configuration {
		configMap[k] = v
	}
	var diags diag.Diagnostics
	data.Configuration, diags = types.MapValueFrom(ctx, types.DynamicType, configMap)
	resp.Diagnostics.Append(diags...)

	// Read extractors
	if exList, err := r.client.ListInputExtractors(data.ID.ValueString()); err == nil {
		// Normalize into list of objects with "data" map to keep stable schema
		var items []map[string]interface{}
		for _, ex := range exList {
			items = append(items, map[string]interface{}{"data": ex})
		}
		data.Extractors, diags = types.ListValueFrom(ctx, types.MapType{ElemType: types.DynamicType}, items)
		resp.Diagnostics.Append(diags...)
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

	// Apply timeout
	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Convert configuration map (allow dynamic values)
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() {
		tmp := make(map[string]interface{}, len(data.Configuration.Elements()))
		diags = data.Configuration.ElementsAs(ctx, &tmp, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range tmp {
			config[k] = v
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
	if !data.Extractors.IsNull() && !data.Extractors.IsUnknown() {
		var extractorObjs []map[string]interface{}
		diags = data.Extractors.ElementsAs(ctx, &extractorObjs, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
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
