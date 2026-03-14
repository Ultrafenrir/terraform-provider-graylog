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

// graylog_index_sets — список index sets + map(title->id)
type indexSetsListDataSource struct{ client *client.Client }

type indexSetsListModel struct {
	Items    types.List `tfsdk:"items"`
	TitleMap types.Map  `tfsdk:"title_map"`
}

func NewIndexSetsListDataSource() datasource.DataSource { return &indexSetsListDataSource{} }

func (d *indexSetsListDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_index_sets"
}

func (d *indexSetsListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Graylog index sets and returns both full items and a title->id map.",
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of index sets with selected fields.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"title":        schema.StringAttribute{Computed: true},
						"description":  schema.StringAttribute{Computed: true},
						"index_prefix": schema.StringAttribute{Computed: true},
						"default":      schema.BoolAttribute{Computed: true},
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

func (d *indexSetsListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *indexSetsListDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.WithContext(ctx).ListIndexSets()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list index sets", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":           types.StringType,
		"title":        types.StringType,
		"description":  types.StringType,
		"index_prefix": types.StringType,
		"default":      types.BoolType,
	}
	itemVals := make([]attr.Value, 0, len(list))
	m := make(map[string]attr.Value)
	for _, s := range list {
		obj, di := types.ObjectValue(itemType, map[string]attr.Value{
			"id":           types.StringValue(s.ID),
			"title":        types.StringValue(s.Title),
			"description":  types.StringValue(s.Description),
			"index_prefix": types.StringValue(s.IndexPrefix),
			"default":      types.BoolValue(s.Default),
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
	out := indexSetsListModel{Items: items, TitleMap: mp}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
