package provider

import (
	"context"
	"errors"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type indexSetDataSource struct{ client *client.Client }

type indexSetDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Title             types.String `tfsdk:"title"`
	Description       types.String `tfsdk:"description"`
	IndexPrefix       types.String `tfsdk:"index_prefix"`
	Shards            types.Int64  `tfsdk:"shards"`
	Replicas          types.Int64  `tfsdk:"replicas"`
	RotationStrategy  types.String `tfsdk:"rotation_strategy"`
	RetentionStrategy types.String `tfsdk:"retention_strategy"`
	IndexAnalyzer     types.String `tfsdk:"index_analyzer"`
	Default           types.Bool   `tfsdk:"default"`
}

func NewIndexSetDataSource() datasource.DataSource { return &indexSetDataSource{} }

func (d *indexSetDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_index_set"
}

func (d *indexSetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Graylog index set by ID. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Required: true, Description: "The unique identifier of the index set"},
			"title":              schema.StringAttribute{Computed: true, Description: "The title of the index set"},
			"description":        schema.StringAttribute{Computed: true, Description: "Description of the index set"},
			"index_prefix":       schema.StringAttribute{Computed: true, Description: "Index name prefix"},
			"shards":             schema.Int64Attribute{Computed: true, Description: "Number of Elasticsearch shards"},
			"replicas":           schema.Int64Attribute{Computed: true, Description: "Number of Elasticsearch replicas"},
			"rotation_strategy":  schema.StringAttribute{Computed: true, Description: "Index rotation strategy"},
			"retention_strategy": schema.StringAttribute{Computed: true, Description: "Index retention strategy"},
			"index_analyzer":     schema.StringAttribute{Computed: true, Description: "Elasticsearch analyzer"},
			"default":            schema.BoolAttribute{Computed: true, Description: "Whether this is the default index set"},
		},
	}
}

func (d *indexSetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *indexSetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data indexSetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	is, err := d.client.GetIndexSet(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Index set not found", "The index set with the specified ID was not found")
			return
		}
		resp.Diagnostics.AddError("Error reading index set", err.Error())
		return
	}

	data.Title = types.StringValue(is.Title)
	data.Description = types.StringValue(is.Description)
	data.IndexPrefix = types.StringValue(is.IndexPrefix)
	data.Shards = types.Int64Value(int64(is.Shards))
	data.Replicas = types.Int64Value(int64(is.Replicas))
	data.RotationStrategy = types.StringValue(is.RotationStrategy)
	data.RetentionStrategy = types.StringValue(is.RetentionStrategy)
	data.IndexAnalyzer = types.StringValue(is.IndexAnalyzer)
	data.Default = types.BoolValue(is.Default)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
