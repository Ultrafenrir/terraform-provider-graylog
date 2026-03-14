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

// graylog_inputs — список инпутов + map(title->id)
type inputsListDataSource struct{ client *client.Client }

type inputsListModel struct {
	Items    types.List `tfsdk:"items"`
	TitleMap types.Map  `tfsdk:"title_map"`
}

func NewInputsListDataSource() datasource.DataSource { return &inputsListDataSource{} }

func (d *inputsListDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_inputs"
}

func (d *inputsListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Graylog inputs and returns both full items and a title->id map.",
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of inputs with selected fields.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":     schema.StringAttribute{Computed: true},
						"title":  schema.StringAttribute{Computed: true},
						"type":   schema.StringAttribute{Computed: true},
						"global": schema.BoolAttribute{Computed: true},
						"node":   schema.StringAttribute{Computed: true},
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

func (d *inputsListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *inputsListDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.WithContext(ctx).ListInputs()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list inputs", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":     types.StringType,
		"title":  types.StringType,
		"type":   types.StringType,
		"global": types.BoolType,
		"node":   types.StringType,
	}
	itemVals := make([]attr.Value, 0, len(list))
	m := make(map[string]attr.Value)
	for _, it := range list {
		obj, di := types.ObjectValue(itemType, map[string]attr.Value{
			"id":     types.StringValue(it.ID),
			"title":  types.StringValue(it.Title),
			"type":   types.StringValue(it.Type),
			"global": types.BoolValue(it.Global),
			"node":   types.StringValue(it.Node),
		})
		resp.Diagnostics.Append(di...)
		itemVals = append(itemVals, obj)
		m[it.Title] = types.StringValue(it.ID)
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
	out := inputsListModel{Items: items, TitleMap: mp}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
