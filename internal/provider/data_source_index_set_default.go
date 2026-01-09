package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type indexSetDefaultDataSource struct{ client *client.Client }

type indexSetDefaultDSModel struct {
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

func NewIndexSetDefaultDataSource() datasource.DataSource { return &indexSetDefaultDataSource{} }

func (d *indexSetDefaultDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_index_set_default"
}

func (d *indexSetDefaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the default (writable) Graylog index set.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, Description: "The unique identifier of the default index set"},
			"title":              schema.StringAttribute{Computed: true},
			"description":        schema.StringAttribute{Computed: true},
			"index_prefix":       schema.StringAttribute{Computed: true},
			"shards":             schema.Int64Attribute{Computed: true},
			"replicas":           schema.Int64Attribute{Computed: true},
			"rotation_strategy":  schema.StringAttribute{Computed: true},
			"retention_strategy": schema.StringAttribute{Computed: true},
			"index_analyzer":     schema.StringAttribute{Computed: true},
			"default":            schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *indexSetDefaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *indexSetDefaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data indexSetDefaultDSModel
	// no config to read; keep for uniformity

	sets, err := d.client.ListIndexSets()
	if err != nil {
		resp.Diagnostics.AddError("Error listing index sets", err.Error())
		return
	}
	for _, s := range sets {
		if s.Default {
			data.ID = types.StringValue(s.ID)
			data.Title = types.StringValue(s.Title)
			data.Description = types.StringValue(s.Description)
			data.IndexPrefix = types.StringValue(s.IndexPrefix)
			data.Shards = types.Int64Value(int64(s.Shards))
			data.Replicas = types.Int64Value(int64(s.Replicas))
			data.RotationStrategy = types.StringValue(s.RotationStrategy)
			data.RetentionStrategy = types.StringValue(s.RetentionStrategy)
			data.IndexAnalyzer = types.StringValue(s.IndexAnalyzer)
			data.Default = types.BoolValue(s.Default)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("Default index set not found", "Could not find index set with default=true")
}
