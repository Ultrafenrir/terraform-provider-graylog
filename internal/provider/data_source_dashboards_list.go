package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// graylog_dashboards — список дашбордов + map(title->id)
type dashboardsListDataSource struct{ client *client.Client }

type dashboardsListModel struct {
	Items    types.List `tfsdk:"items"`
	TitleMap types.Map  `tfsdk:"title_map"`
}

func NewDashboardsListDataSource() datasource.DataSource { return &dashboardsListDataSource{} }

func (d *dashboardsListDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_dashboards"
}

func (d *dashboardsListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Graylog classic dashboards and returns both full items and a title->id map.",
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of dashboards with selected fields.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"title":       schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
					},
				},
			},
			"title_map": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "A convenience map of title -> id (exact titles).",
			},
		},
	}
}

func (d *dashboardsListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *dashboardsListDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.WithContext(ctx).ListDashboards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list dashboards", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":          types.StringType,
		"title":       types.StringType,
		"description": types.StringType,
	}
	itemVals := make([]attr.Value, 0, len(list))
	m := make(map[string]attr.Value)
	for _, s := range list {
		obj, di := types.ObjectValue(itemType, map[string]attr.Value{
			"id":          types.StringValue(s.ID),
			"title":       types.StringValue(s.Title),
			"description": types.StringValue(s.Description),
		})
		resp.Diagnostics.Append(di...)
		itemVals = append(itemVals, obj)
		m[s.Title] = types.StringValue(s.ID)
	}
	var dgs diag.Diagnostics
	items, d1 := types.ListValue(types.ObjectType{AttrTypes: itemType}, itemVals)
	mp, d2 := types.MapValue(types.StringType, m)
	dgs.Append(d1...)
	dgs.Append(d2...)
	resp.Diagnostics.Append(dgs...)
	if resp.Diagnostics.HasError() {
		return
	}
	out := dashboardsListModel{Items: items, TitleMap: mp}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
