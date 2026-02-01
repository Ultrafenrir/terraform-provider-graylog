package provider

import (
	"context"
	"errors"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type streamDataSource struct{ client *client.Client }

type streamDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Disabled    types.Bool   `tfsdk:"disabled"`
	IndexSetID  types.String `tfsdk:"index_set_id"`
}

func NewStreamDataSource() datasource.DataSource { return &streamDataSource{} }

func (d *streamDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_stream"
}

func (d *streamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Graylog stream by ID. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Required: true, Description: "The unique identifier of the stream"},
			"title":        schema.StringAttribute{Computed: true, Description: "The title of the stream"},
			"description":  schema.StringAttribute{Computed: true, Description: "Description of the stream"},
			"disabled":     schema.BoolAttribute{Computed: true, Description: "Whether the stream is disabled"},
			"index_set_id": schema.StringAttribute{Computed: true, Description: "The index set ID associated with this stream"},
		},
	}
}

func (d *streamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *streamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data streamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := d.client.WithContext(ctx).GetStream(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Stream not found", "The stream with the specified ID was not found")
			return
		}
		resp.Diagnostics.AddError("Error reading stream", err.Error())
		return
	}

	data.Title = types.StringValue(s.Title)
	data.Description = types.StringValue(s.Description)
	data.Disabled = types.BoolValue(s.Disabled)
	data.IndexSetID = types.StringValue(s.IndexSetID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
