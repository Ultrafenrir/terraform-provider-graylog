package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type userDataSource struct{ client *client.Client }

type userDataModel struct {
	Username         types.String `tfsdk:"username"`
	FullName         types.String `tfsdk:"full_name"`
	Email            types.String `tfsdk:"email"`
	Roles            types.List   `tfsdk:"roles"`
	Timezone         types.String `tfsdk:"timezone"`
	SessionTimeoutMs types.Int64  `tfsdk:"session_timeout_ms"`
	Disabled         types.Bool   `tfsdk:"disabled"`
}

func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

func (d *userDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lookup Graylog user by username",
		Attributes: map[string]schema.Attribute{
			"username":           schema.StringAttribute{Required: true, Description: "Username of the user"},
			"full_name":          schema.StringAttribute{Computed: true},
			"email":              schema.StringAttribute{Computed: true},
			"roles":              schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"timezone":           schema.StringAttribute{Computed: true},
			"session_timeout_ms": schema.Int64Attribute{Computed: true},
			"disabled":           schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u, err := d.client.GetUser(data.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("username"), "User not found", err.Error())
		return
	}

	data.FullName = types.StringValue(u.FullName)
	data.Email = types.StringValue(u.Email)
	data.Timezone = types.StringValue(u.Timezone)
	data.SessionTimeoutMs = types.Int64Value(u.SessionTimeoutMs)
	data.Disabled = types.BoolValue(u.Disabled)
	// roles
	elems := make([]attr.Value, 0, len(u.Roles))
	for _, r := range u.Roles {
		elems = append(elems, types.StringValue(r))
	}
	rl, di := types.ListValue(types.StringType, elems)
	resp.Diagnostics.Append(di...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Roles = rl

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
