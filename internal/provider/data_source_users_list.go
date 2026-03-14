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

// graylog_users — список пользователей + map(username->id|username)
type usersListDataSource struct{ client *client.Client }

type usersListModel struct {
	Items    types.List `tfsdk:"items"`
	TitleMap types.Map  `tfsdk:"title_map"`
}

func NewUsersListDataSource() datasource.DataSource { return &usersListDataSource{} }

func (d *usersListDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_users"
}

func (d *usersListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Graylog users and returns both full items and a username->id map (id may be empty on older versions).",
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of users with selected fields.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":        schema.StringAttribute{Computed: true},
						"username":  schema.StringAttribute{Computed: true},
						"full_name": schema.StringAttribute{Computed: true},
						"email":     schema.StringAttribute{Computed: true},
						"disabled":  schema.BoolAttribute{Computed: true},
					},
				},
			},
			"title_map": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "A convenience map of username -> id (or username on versions without ids).",
			},
		},
	}
}

func (d *usersListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *usersListDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.WithContext(ctx).ListUsers()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list users", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":        types.StringType,
		"username":  types.StringType,
		"full_name": types.StringType,
		"email":     types.StringType,
		"disabled":  types.BoolType,
	}
	itemVals := make([]attr.Value, 0, len(list))
	m := make(map[string]attr.Value)
	for _, it := range list {
		obj, di := types.ObjectValue(itemType, map[string]attr.Value{
			"id":        types.StringValue(it.ID),
			"username":  types.StringValue(it.Username),
			"full_name": types.StringValue(it.FullName),
			"email":     types.StringValue(it.Email),
			"disabled":  types.BoolValue(it.Disabled),
		})
		resp.Diagnostics.Append(di...)
		itemVals = append(itemVals, obj)
		val := it.ID
		if val == "" {
			val = it.Username
		}
		m[it.Username] = types.StringValue(val)
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
	out := usersListModel{Items: items, TitleMap: mp}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
