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

type pipelineResource struct{ client *client.Client }

type pipelineModel struct {
	ID          types.String   `tfsdk:"id"`
	Title       types.String   `tfsdk:"title"`
	Description types.String   `tfsdk:"description"`
	Source      types.String   `tfsdk:"source"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewPipelineResource() resource.Resource { return &pipelineResource{} }

func (r *pipelineResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_pipeline"
}

func (r *pipelineResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a Graylog pipeline",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Pipeline ID"},
			"title":       schema.StringAttribute{Required: true, Description: "Pipeline title"},
			"description": schema.StringAttribute{Optional: true, Description: "Pipeline description"},
			"source":      schema.StringAttribute{Optional: true, Description: "Pipeline source definition"},
			"timeouts":    timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *pipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *pipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data pipelineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validatePipeline(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	created, err := r.client.CreatePipeline(&client.Pipeline{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Source:      data.Source.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating pipeline", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data pipelineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := r.client.GetPipeline(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading pipeline", err.Error())
		return
	}
	data.Title = types.StringValue(p.Title)
	data.Description = types.StringValue(p.Description)
	data.Source = types.StringValue(p.Source)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pipelineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data pipelineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validatePipeline(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	_, err := r.client.UpdatePipeline(data.ID.ValueString(), &client.Pipeline{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Source:      data.Source.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating pipeline", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// validatePipeline checks title presence and warns about empty source.
func validatePipeline(m *pipelineModel) (d diag.Diagnostics) {
	if m.Title.IsNull() || m.Title.IsUnknown() || m.Title.ValueString() == "" {
		d.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if m.Source.IsNull() || m.Source.IsUnknown() || m.Source.ValueString() == "" {
		d.AddAttributeWarning(path.Root("source"), "Empty pipeline source", "Attribute 'source' is empty; pipeline with no stages/rules may have no effect.")
	}
	return
}

func (r *pipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data pipelineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := data.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	if err := r.client.DeletePipeline(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting pipeline", err.Error())
	}
}

func (r *pipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
