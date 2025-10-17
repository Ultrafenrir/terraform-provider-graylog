package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type streamResource struct{ client *client.Client }

type streamRuleModel struct {
	Field types.String `tfsdk:"field"`
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type streamModel struct {
	ID          types.String      `tfsdk:"id"`
	Title       types.String      `tfsdk:"title"`
	Description types.String      `tfsdk:"description"`
	Disabled    types.Bool        `tfsdk:"disabled"`
	IndexSetID  types.String      `tfsdk:"index_set_id"`
	Rules       []streamRuleModel `tfsdk:"rule"`
}

func NewStreamResource() resource.Resource { return &streamResource{} }

func (r *streamResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_stream"
}

func (r *streamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"title":        schema.StringAttribute{Required: true},
			"description":  schema.StringAttribute{Optional: true},
			"disabled":     schema.BoolAttribute{Optional: true},
			"index_set_id": schema.StringAttribute{Optional: true},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				Description: "Stream rules",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{Required: true},
						"type":  schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *streamResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil { return }
	r.client = req.ProviderData.(*client.Client)
}

func (r *streamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	var rules []client.Rule
	for _, rr := range data.Rules {
		rules = append(rules, client.Rule{Field: rr.Field.ValueString(), Type: rr.Type.ValueString(), Value: rr.Value.ValueString()})
	}
	created, err := r.client.CreateStream(&client.Stream{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Disabled:    data.Disabled.ValueBool(),
		IndexSetID:  data.IndexSetID.ValueString(),
		Rules:       rules,
	})
	if err != nil { resp.Diagnostics.AddError("Error creating stream", err.Error()); return }
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	s, err := r.client.GetStream(data.ID.ValueString())
	if err != nil { resp.Diagnostics.AddError("Error reading stream", err.Error()); return }
	data.Title = types.StringValue(s.Title)
	data.Description = types.StringValue(s.Description)
	data.Disabled = types.BoolValue(s.Disabled)
	data.IndexSetID = types.StringValue(s.IndexSetID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	if err := r.client.DeleteStream(data.ID.ValueString()); err != nil { resp.Diagnostics.AddError("Error deleting stream", err.Error()) }
}
