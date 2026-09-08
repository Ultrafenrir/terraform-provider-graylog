package provider

import (
	"context"
	"errors"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type roleDataSource struct{ client *client.Client }

type roleDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permissions types.List   `tfsdk:"permissions"`
	ReadOnly    types.Bool   `tfsdk:"read_only"`
}

func NewRoleDataSource() datasource.DataSource { return &roleDataSource{} }

func (d *roleDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_role"
}

func (d *roleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a Graylog role by name, including built-in roles such as `Reader`. " +
			"Use it wherever an API wants a role id rather than a role name — `graylog_auth_backend`'s " +
			"`default_roles` is the common case.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				Description: "Role id. This is the value APIs that take role identifiers expect; several of " +
					"them accept a name without complaint and then misbehave, so prefer this.",
			},
			"name":        schema.StringAttribute{Required: true, Description: "Exact role name."},
			"description": schema.StringAttribute{Computed: true, Description: "Role description."},
			"permissions": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Permissions granted by the role."},
			"read_only":   schema.BoolAttribute{Computed: true, Description: "Whether this is a built-in role."},
		},
	}
}

func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data roleDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := d.client.WithContext(ctx).GetRoleByName(data.Name.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Role not found",
				"No role named "+data.Name.ValueString()+" exists on this Graylog server.")
			return
		}
		resp.Diagnostics.AddError("Error reading role", err.Error())
		return
	}

	permissions, diags := types.ListValueFrom(ctx, types.StringType, role.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(role.ID)
	data.Description = types.StringValue(role.Description)
	data.Permissions = permissions
	data.ReadOnly = types.BoolValue(role.ReadOnly)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
