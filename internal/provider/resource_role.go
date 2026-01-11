package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type roleResource struct{ client *client.Client }

type roleModel struct {
	ID          types.String   `tfsdk:"id"` // store name as id
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Permissions []types.String `tfsdk:"permissions"`
	ReadOnly    types.Bool     `tfsdk:"read_only"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewRoleResource() resource.Resource { return &roleResource{} }

func (r *roleResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_role"
}

func (r *roleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Graylog Role.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Role identifier (role name)"},
			"name":        schema.StringAttribute{Required: true, Description: "Role name (immutable)"},
			"description": schema.StringAttribute{Optional: true, Description: "Description"},
			"permissions": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "List of permissions"},
			"read_only":   schema.BoolAttribute{Computed: true, Description: "Read-only system role"},
			"timeouts":    timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	perms := make([]string, 0, len(data.Permissions))
	for _, p := range data.Permissions {
		if p.IsNull() || p.IsUnknown() || p.ValueString() == "" {
			continue
		}
		perms = append(perms, p.ValueString())
	}
	created, err := r.client.CreateRole(&client.Role{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Permissions: perms,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating role", err.Error())
		return
	}
	data.ID = types.StringValue(created.Name)
	data.ReadOnly = types.BoolValue(created.ReadOnly)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	if name == "" {
		name = data.ID.ValueString()
	}
	ro, err := r.client.GetRole(name)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role", err.Error())
		return
	}
	data.ID = types.StringValue(ro.Name)
	data.Name = types.StringValue(ro.Name)
	data.Description = types.StringValue(ro.Description)
	data.ReadOnly = types.BoolValue(ro.ReadOnly)
	perms := make([]types.String, 0, len(ro.Permissions))
	for _, p := range ro.Permissions {
		perms = append(perms, types.StringValue(p))
	}
	data.Permissions = perms
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	perms := make([]string, 0, len(data.Permissions))
	for _, p := range data.Permissions {
		if p.IsNull() || p.IsUnknown() || p.ValueString() == "" {
			continue
		}
		perms = append(perms, p.ValueString())
	}
	if _, err := r.client.UpdateRole(data.Name.ValueString(), &client.Role{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Permissions: perms,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRole(data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting role", err.Error())
		return
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// import by name
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
