package provider

import (
	"context"
	"fmt"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type roleResource struct{ client *client.Client }

type roleModel struct {
	ID          types.String   `tfsdk:"id"` // store name as id
	RoleID      types.String   `tfsdk:"role_id"`
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
			"id": schema.StringAttribute{Computed: true, Description: "Role identifier (role name)"},
			"role_id": schema.StringAttribute{
				Computed: true,
				Description: "Mongo id of the role. Several APIs take a role id and reject nothing when given " +
					"a name — an authentication backend whose default_roles carry names is stored without " +
					"complaint and then fails every login — so pass this where an id is wanted.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
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

// resolveRoleID looks up the Mongo id, which the name-keyed role endpoints do
// not return. /authz/roles needs the same roles:read as GetRole, so a failure
// here is a real error: a null id would reach default_roles as an invalid
// element, and on Update it would contradict the id the plan already holds.
func (r *roleResource) resolveRoleID(ctx context.Context, name string) (types.String, error) {
	role, err := r.client.WithContext(ctx).GetRoleByName(name)
	if err != nil {
		return types.StringNull(), err
	}
	if role.ID == "" {
		return types.StringNull(), fmt.Errorf("role %q has no id", name)
	}
	return types.StringValue(role.ID), nil
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
	created, err := r.client.WithContext(ctx).CreateRole(&client.Role{
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
	data.RoleID, err = r.resolveRoleID(ctx, created.Name)
	if err != nil {
		// The role exists on the server now; save state so Terraform taints
		// and replaces it instead of orphaning it.
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		resp.Diagnostics.AddError("Error resolving role id", err.Error())
		return
	}
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
	ro, err := r.client.WithContext(ctx).GetRole(name)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role", err.Error())
		return
	}
	data.ID = types.StringValue(ro.Name)
	data.Name = types.StringValue(ro.Name)
	data.Description = types.StringValue(ro.Description)
	data.ReadOnly = types.BoolValue(ro.ReadOnly)
	data.RoleID, err = r.resolveRoleID(ctx, ro.Name)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving role id", err.Error())
		return
	}
	// Graylog stores permissions as a set and returns it in arbitrary order;
	// keep the prior state ordering when the sets are equal to avoid phantom
	// reorder diffs on every plan.
	if !samePermissionSet(data.Permissions, ro.Permissions) {
		perms := make([]types.String, 0, len(ro.Permissions))
		for _, p := range ro.Permissions {
			perms = append(perms, types.StringValue(p))
		}
		data.Permissions = perms
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// samePermissionSet reports whether the state permission list and the API
// permission list contain the same elements (order-insensitive, multiset).
func samePermissionSet(state []types.String, api []string) bool {
	if len(state) != len(api) {
		return false
	}
	counts := make(map[string]int, len(api))
	for _, p := range api {
		counts[p]++
	}
	for _, s := range state {
		counts[s.ValueString()]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
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
	if _, err := r.client.WithContext(ctx).UpdateRole(data.Name.ValueString(), &client.Role{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Permissions: perms,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating role", err.Error())
		return
	}
	// Ensure all computed fields are known after apply
	if ro, err := r.client.WithContext(ctx).GetRole(data.Name.ValueString()); err == nil {
		data.ReadOnly = types.BoolValue(ro.ReadOnly)
		data.ID = types.StringValue(ro.Name)
	} else {
		// Fallback to false if read fails; will be corrected on next Read
		data.ReadOnly = types.BoolValue(false)
		data.ID = types.StringValue(data.Name.ValueString())
	}
	roleID, err := r.resolveRoleID(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving role id", err.Error())
		return
	}
	data.RoleID = roleID
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.WithContext(ctx).DeleteRole(data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting role", err.Error())
		return
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// import by name
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
