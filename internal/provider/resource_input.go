package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type inputResource struct{ client *client.Client }

type inputModel struct {
	ID           types.String   `tfsdk:"id"`
	Title        types.String   `tfsdk:"title"`
	Type         types.String   `tfsdk:"type"`
	BindAddress  types.String   `tfsdk:"bind_address"`
	Port         types.Int64    `tfsdk:"port"`
	KafkaBrokers []types.String `tfsdk:"kafka_brokers"`
	Topic        types.String   `tfsdk:"topic"`
	IndexSetID   types.String   `tfsdk:"index_set_id"`
}

func NewInputResource() resource.Resource { return &inputResource{} }

func (r *inputResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_input"
}

func (r *inputResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"title":         schema.StringAttribute{Required: true},
			"type":          schema.StringAttribute{Required: true},
			"bind_address":  schema.StringAttribute{Optional: true},
			"port":          schema.Int64Attribute{Optional: true},
			"topic":         schema.StringAttribute{Optional: true},
			"index_set_id":  schema.StringAttribute{Optional: true},
			"kafka_brokers": schema.ListAttribute{ElementType: types.StringType, Optional: true},
		},
	}
}

func (r *inputResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil { return }
	r.client = req.ProviderData.(*client.Client)
}

func (r *inputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	var brokers []string
	for _, b := range data.KafkaBrokers { brokers = append(brokers, b.ValueString()) }
	in := &client.Input{
		Title:        data.Title.ValueString(),
		Type:         data.Type.ValueString(),
		BindAddress:  data.BindAddress.ValueString(),
		Port:         int(data.Port.ValueInt64()),
		Topic:        data.Topic.ValueString(),
		IndexSetID:   data.IndexSetID.ValueString(),
		KafkaBrokers: brokers,
	}
	created, err := r.client.CreateInput(in)
	if err != nil { resp.Diagnostics.AddError("Error creating input", err.Error()); return }
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() { return }
}
