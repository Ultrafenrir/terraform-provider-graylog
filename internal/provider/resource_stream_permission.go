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

// Resource to manage role-based permissions for a specific stream.
// It ensures that the target role contains exactly the requested set of
// stream-scoped permissions (read/edit/share) for the given stream ID,
// without touching other unrelated permissions of the role.
type streamPermissionResource struct{ client *client.Client }

type streamPermissionModel struct {
	ID       types.String   `tfsdk:"id"`
	RoleName types.String   `tfsdk:"role_name"`
	StreamID types.String   `tfsdk:"stream_id"`
	Actions  []types.String `tfsdk:"actions"` // allowed: read, edit, share
}

func NewStreamPermissionResource() resource.Resource { return &streamPermissionResource{} }

func (r *streamPermissionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_stream_permission"
}

func (r *streamPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages role-based permissions (read/edit/share) for a specific Graylog Stream.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, Description: "Synthetic ID: <role_name>/<stream_id>"},
			"role_name": schema.StringAttribute{Required: true, Description: "Target role name"},
			"stream_id": schema.StringAttribute{Required: true, Description: "Stream ID"},
			"actions": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "List of actions to grant for the stream: read, edit, share (default: [read])",
			},
		},
	}
}

func (r *streamPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *streamPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data streamPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRolePermissions(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error applying stream permissions to role", err.Error())
		return
	}
	data.ID = types.StringValue(syntheticPermID(data.RoleName.ValueString(), data.StreamID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data streamPermissionModel
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
	// Detect which stream-scoped actions are currently present
	present := detectActionsForStream(role.Permissions, data.StreamID.ValueString())
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
	data.ID = types.StringValue(syntheticPermID(data.RoleName.ValueString(), data.StreamID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data streamPermissionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyRolePermissions(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error updating stream permissions on role", err.Error())
		return
	}
	data.ID = types.StringValue(syntheticPermID(data.RoleName.ValueString(), data.StreamID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data streamPermissionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role, err := r.client.WithContext(ctx).GetRole(data.RoleName.ValueString())
	if err != nil || role == nil {
		return
	}
	// Remove any stream-scoped permissions for this stream
	filtered := filterOutStreamPerms(role.Permissions, data.StreamID.ValueString())
	if _, err := r.client.WithContext(ctx).UpdateRole(role.Name, &client.Role{
		Description: role.Description,
		Permissions: filtered,
		ReadOnly:    role.ReadOnly,
	}); err != nil {
		resp.Diagnostics.AddError("Error removing stream permissions from role", err.Error())
		return
	}
}

func (r *streamPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	raw := strings.TrimSpace(req.ID)
	if raw == "" {
		resp.Diagnostics.AddError("Empty import ID", "Provide '<role_name>/<stream_id>' or '<role_name>:<stream_id>'.")
		return
	}
	rn, sid := parseTwoPart(raw)
	if rn == "" || sid == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected '<role_name>/<stream_id>' or '<role_name>:<stream_id>'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), syntheticPermID(rn, sid))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_name"), rn)...)  //nolint:errcheck
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("stream_id"), sid)...) //nolint:errcheck
}

// --- helpers ---

func (r *streamPermissionResource) applyRolePermissions(ctx context.Context, m *streamPermissionModel) error {
	role, err := r.client.WithContext(ctx).GetRole(m.RoleName.ValueString())
	if err != nil {
		return err
	}
	if role == nil || role.Name == "" {
		return fmt.Errorf("role not found: %s", m.RoleName.ValueString())
	}
	desired := toPermStrings(m.StreamID.ValueString(), toActionStrings(m.Actions))
	// remove existing stream-scoped perms for this stream, then add desired
	base := filterOutStreamPerms(role.Permissions, m.StreamID.ValueString())
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

func toActionStrings(in []types.String) []string {
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

func toPermStrings(streamID string, actions []string) []string {
	res := make([]string, 0, len(actions))
	for _, a := range actions {
		res = append(res, fmt.Sprintf("streams:%s:%s", a, streamID))
	}
	return res
}

func filterOutStreamPerms(perms []string, streamID string) []string {
	out := make([]string, 0, len(perms))
	needle := ":" + streamID
	for _, p := range perms {
		if strings.HasPrefix(p, "streams:") && strings.HasSuffix(p, needle) {
			// skip this stream-scoped perm (we will re-add desired ones)
			continue
		}
		out = append(out, p)
	}
	return out
}

func detectActionsForStream(perms []string, streamID string) []string {
	got := make(map[string]struct{})
	needle := ":" + streamID
	for _, p := range perms {
		if strings.HasPrefix(p, "streams:") && strings.HasSuffix(p, needle) {
			// expect format streams:<action>:<id>
			parts := strings.SplitN(p, ":", 3)
			if len(parts) == 3 {
				act := parts[1]
				if act == "read" || act == "edit" || act == "share" {
					got[act] = struct{}{}
				}
			}
		}
	}
	res := make([]string, 0, len(got))
	for k := range got {
		res = append(res, k)
	}
	return res
}

func syntheticPermID(roleName, streamID string) string { return roleName + "/" + streamID }

func parseTwoPart(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return "", ""
}
