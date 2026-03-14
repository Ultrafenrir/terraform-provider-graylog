package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Resource to manage role-based permissions for a specific dashboard.
// It ensures that the target role contains exactly the requested set of
// dashboard-scoped permissions (read/edit/share) for the given dashboard ID,
// without touching other unrelated permissions of the role.
type dashboardPermissionResource struct{ client *client.Client }

type dashboardPermissionModel struct {
	ID          types.String   `tfsdk:"id"`
	RoleName    types.String   `tfsdk:"role_name"`
	DashboardID types.String   `tfsdk:"dashboard_id"`
	Actions     []types.String `tfsdk:"actions"` // allowed: read, edit, share
}

func NewDashboardPermissionResource() resource.Resource { return &dashboardPermissionResource{} }

func (r *dashboardPermissionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_dashboard_permission"
}

func (r *dashboardPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages role-based permissions (read/edit/share) for a specific Graylog Dashboard.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, Description: "Synthetic ID: <role_name>/<dashboard_id>"},
			"role_name":    schema.StringAttribute{Required: true, Description: "Target role name"},
			"dashboard_id": schema.StringAttribute{Required: true, Description: "Dashboard ID"},
			"actions":      schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "List of actions to grant for the dashboard: read, edit, share (default: [read])"},
		},
	}
}

func (r *dashboardPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *dashboardPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dashboardPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Capability gating: classic dashboards CRUD/permissions must be supported by this instance
	caps := r.client.GetCapabilities()
	resp.Diagnostics.Append(ensureFeature(ctx, r.client, caps.ClassicDashboardsCRUD, "classic_dashboards_crud", "try Graylog 7.x or appropriate image")...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRolePermissions(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error applying dashboard permissions to role", err.Error())
		return
	}
	data.ID = types.StringValue(dashboardSyntheticPermID(data.RoleName.ValueString(), data.DashboardID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dashboardPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role, err := r.client.WithContext(ctx).GetRole(data.RoleName.ValueString())
	if err != nil || role == nil {
		// Role missing — consider binding removed
		resp.State.RemoveResource(ctx)
		return
	}
	// Detect which dashboard-scoped actions are currently present
	present := detectActionsForDashboard(role.Permissions, data.DashboardID.ValueString())
	if len(present) == 0 {
		// No relevant permissions — resource considered gone
		resp.State.RemoveResource(ctx)
		return
	}
	// Keep in stable order
	sort.Strings(present)
	data.Actions = make([]types.String, 0, len(present))
	for _, a := range present {
		data.Actions = append(data.Actions, types.StringValue(a))
	}
	data.ID = types.StringValue(dashboardSyntheticPermID(data.RoleName.ValueString(), data.DashboardID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dashboardPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Capability gating: classic dashboards CRUD/permissions must be supported by this instance
	caps := r.client.GetCapabilities()
	resp.Diagnostics.Append(ensureFeature(ctx, r.client, caps.ClassicDashboardsCRUD, "classic_dashboards_crud", "try Graylog 7.x or appropriate image")...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRolePermissions(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error updating dashboard permissions on role", err.Error())
		return
	}
	data.ID = types.StringValue(dashboardSyntheticPermID(data.RoleName.ValueString(), data.DashboardID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data dashboardPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role, err := r.client.WithContext(ctx).GetRole(data.RoleName.ValueString())
	if err != nil || role == nil {
		return
	}
	// Remove any dashboard-scoped permissions for this dashboard
	filtered := filterOutDashboardPerms(role.Permissions, data.DashboardID.ValueString())
	if _, err := r.client.WithContext(ctx).UpdateRole(role.Name, &client.Role{
		Description: role.Description,
		Permissions: filtered,
		ReadOnly:    role.ReadOnly,
	}); err != nil {
		resp.Diagnostics.AddError("Error removing dashboard permissions from role", err.Error())
		return
	}
}

func (r *dashboardPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	raw := strings.TrimSpace(req.ID)
	if raw == "" {
		resp.Diagnostics.AddError("Empty import ID", "Provide '<role_name>/<dashboard_id>' or '<role_name>:<dashboard_id>'.")
		return
	}
	rn, did := parseTwoPartDash(raw)
	if rn == "" || did == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected '<role_name>/<dashboard_id>' or '<role_name>:<dashboard_id>'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), dashboardSyntheticPermID(rn, did))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_name"), rn)...)     //nolint:errcheck
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dashboard_id"), did)...) //nolint:errcheck
}

// --- helpers ---

func (r *dashboardPermissionResource) applyRolePermissions(ctx context.Context, m *dashboardPermissionModel) error {
	role, err := r.client.WithContext(ctx).GetRole(m.RoleName.ValueString())
	if err != nil {
		return err
	}
	if role == nil || role.Name == "" {
		return fmt.Errorf("role not found: %s", m.RoleName.ValueString())
	}
	desired := toDashboardPermStrings(m.DashboardID.ValueString(), toDashboardActionStrings(m.Actions))
	// remove existing dashboard-scoped perms for this dashboard, then add desired
	base := filterOutDashboardPerms(role.Permissions, m.DashboardID.ValueString())
	merged := append(base, desired...)
	// ensure stable order
	sort.Strings(merged)
	_, err = r.client.WithContext(ctx).UpdateRole(role.Name, &client.Role{
		Description: role.Description,
		Permissions: merged,
		ReadOnly:    role.ReadOnly,
	})
	return err
}

func toDashboardActionStrings(in []types.String) []string {
	if len(in) == 0 {
		return []string{"read"}
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v.IsNull() || v.IsUnknown() {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(v.ValueString()))
		if s == "read" || s == "edit" || s == "share" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		out = []string{"read"}
	}
	// dedupe
	sort.Strings(out)
	j := 0
	for i := 1; i < len(out); i++ {
		if out[i] != out[j] {
			j++
			out[j] = out[i]
		}
	}
	return out[:j+1]
}

func toDashboardPermStrings(dashboardID string, actions []string) []string {
	res := make([]string, 0, len(actions))
	for _, a := range actions {
		res = append(res, fmt.Sprintf("dashboards:%s:%s", a, dashboardID))
	}
	return res
}

func filterOutDashboardPerms(perms []string, dashboardID string) []string {
	if len(perms) == 0 {
		return perms
	}
	needle := ":" + dashboardID
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if strings.HasPrefix(p, "dashboards:") && strings.HasSuffix(p, needle) {
			// skip it
			continue
		}
		out = append(out, p)
	}
	return out
}

func detectActionsForDashboard(perms []string, dashboardID string) []string {
	if len(perms) == 0 {
		return nil
	}
	needle := ":" + dashboardID
	actions := make([]string, 0, 3)
	for _, p := range perms {
		if strings.HasPrefix(p, "dashboards:") && strings.HasSuffix(p, needle) {
			// expect format dashboards:<action>:<id>
			parts := strings.Split(p, ":")
			if len(parts) == 3 && parts[0] == "dashboards" {
				actions = append(actions, parts[1])
			}
		}
	}
	if len(actions) == 0 {
		return nil
	}
	// normalize unique & sorted
	sort.Strings(actions)
	j := 0
	for i := 1; i < len(actions); i++ {
		if actions[i] != actions[j] {
			j++
			actions[j] = actions[i]
		}
	}
	return actions[:j+1]
}

func dashboardSyntheticPermID(roleName string, dashboardID string) string {
	return roleName + "/" + dashboardID
}

func parseTwoPartDash(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if i := strings.IndexByte(s, '/'); i > 0 {
		return s[:i], s[i+1:]
	}
	if i := strings.IndexByte(s, ':'); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", ""
}
