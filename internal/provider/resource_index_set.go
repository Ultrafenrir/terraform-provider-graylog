package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type indexSetResource struct{ client *client.Client }

type indexSetModel struct {
	ID                types.String `tfsdk:"id"`
	Title             types.String `tfsdk:"title"`
	Description       types.String `tfsdk:"description"`
	RotationStrategy  types.String `tfsdk:"rotation_strategy"`
	RetentionStrategy types.String `tfsdk:"retention_strategy"`
	Shards            types.Int64  `tfsdk:"shards"`
}

func NewIndexSetResource() resource.Resource { return &indexSetResource{} }

func (r *indexSetResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_index_set"
}

func (r *indexSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true},
			"title":              schema.StringAttribute{Required: true},
			"description":        schema.StringAttribute{Optional: true},
			"rotation_strategy":  schema.StringAttribute{Optional: true},
			"retention_strategy": schema.StringAttribute{Optional: true},
			"shards":             schema.Int64Attribute{Optional: true},
		},
	}
}

func (r *indexSetResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil { return }
	r.client = req.ProviderData.(*client.Client)
}

func (r *indexSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	created, err := r.client.CreateIndexSet(&client.IndexSet{
		Title:             data.Title.ValueString(),
		Description:       data.Description.ValueString(),
		RotationStrategy:  data.RotationStrategy.ValueString(),
		RetentionStrategy: data.RetentionStrategy.ValueString(),
		Shards:            int(data.Shards.ValueInt64()),
	})
	if err != nil { resp.Diagnostics.AddError("Error creating index set", err.Error()); return }
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
}
