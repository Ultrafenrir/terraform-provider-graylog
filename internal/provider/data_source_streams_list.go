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

// graylog_streams — список стримов + map(title->id)
type streamsListDataSource struct{ client *client.Client }

type streamsListModel struct {
	Items    types.List `tfsdk:"items"`
	TitleMap types.Map  `tfsdk:"title_map"`
}

func NewStreamsListDataSource() datasource.DataSource { return &streamsListDataSource{} }

func (d *streamsListDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_streams"
}

func (d *streamsListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Graylog streams and returns both full items and a title->id map.",
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of streams with selected fields.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"title":        schema.StringAttribute{Computed: true},
						"description":  schema.StringAttribute{Computed: true},
						"disabled":     schema.BoolAttribute{Computed: true},
						"index_set_id": schema.StringAttribute{Computed: true},
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

func (d *streamsListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *streamsListDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.WithContext(ctx).ListStreams()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list streams", err.Error())
		return
	}
	// Build items (list of objects)
	itemType := map[string]attr.Type{
		"id":           types.StringType,
		"title":        types.StringType,
		"description":  types.StringType,
		"disabled":     types.BoolType,
		"index_set_id": types.StringType,
	}
	itemVals := make([]attr.Value, 0, len(list))
	m := make(map[string]attr.Value)
	for _, s := range list {
		obj, di := types.ObjectValue(itemType, map[string]attr.Value{
			"id":           types.StringValue(s.ID),
			"title":        types.StringValue(s.Title),
			"description":  types.StringValue(s.Description),
			"disabled":     types.BoolValue(s.Disabled),
			"index_set_id": types.StringValue(s.IndexSetID),
		})
		resp.Diagnostics.Append(di...)
		itemVals = append(itemVals, obj)
		// Map title->id (последнее значение при дубликатах заголовков)
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
	out := streamsListModel{Items: items, TitleMap: mp}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
