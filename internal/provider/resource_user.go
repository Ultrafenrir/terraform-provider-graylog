package provider

import (
	"context"
	"errors"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type userResource struct{ client *client.Client }

type userModel struct {
	ID               types.String   `tfsdk:"id"`
	Username         types.String   `tfsdk:"username"`
	FullName         types.String   `tfsdk:"full_name"`
	Email            types.String   `tfsdk:"email"`
	Roles            types.List     `tfsdk:"roles"`
	Timezone         types.String   `tfsdk:"timezone"`
	SessionTimeoutMs types.Int64    `tfsdk:"session_timeout_ms"`
	Disabled         types.Bool     `tfsdk:"disabled"`
	Password         types.String   `tfsdk:"password"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func NewUserResource() resource.Resource { return &userResource{} }

func (r *userResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_user"
}

func (r *userResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a Graylog user: a local account, or the pre-created profile of an external (LDAP/AD) user whose roles Terraform owns.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, Description: "Same as username", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"username":  schema.StringAttribute{Required: true, Description: "Username (immutable)"},
			"full_name": schema.StringAttribute{Optional: true, Description: "Full name. Graylog requires at least two words (first and last name) on create. For external (LDAP/AD) users the directory overwrites it on every login; match the directory value or use lifecycle ignore_changes."},
			"email":     schema.StringAttribute{Optional: true, Description: "Email. For external (LDAP/AD) users the directory overwrites it on every login; match the directory value or use lifecycle ignore_changes."},
			"roles":     schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"timezone":  schema.StringAttribute{Optional: true},
			"session_timeout_ms": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Session timeout in milliseconds. When unset Graylog applies its default (8 hours) and the value is read back into state. 0 is rejected: Graylog accepts it but every interactive login then fails.",
				Validators:    []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"disabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Disable the user account. When unset the server's current state is read back and left as it is.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"password": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Sensitive:     true,
				Description:   "Password; required by Graylog on create. Sent to Graylog only when it differs from the value in state, so unrelated updates (e.g. roles) never touch it, which is what makes external (LDAP/AD) users manageable. Removing it from the configuration keeps the state value and is not a change.",
				PlanModifiers: []planmodifier.String{keepStateWhenUnset{}},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// extract roles
	var roles []string
	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	createTimeout, diags := data.Timeouts.Create(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	created, err := r.client.WithContext(ctx).CreateUser(&client.User{
		Username:         data.Username.ValueString(),
		FullName:         data.FullName.ValueString(),
		Email:            data.Email.ValueString(),
		Roles:            roles,
		Timezone:         data.Timezone.ValueString(),
		SessionTimeoutMs: data.SessionTimeoutMs.ValueInt64(),
		Disabled:         data.Disabled.ValueBool(),
		Password:         data.Password.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())
		return
	}
	data.ID = types.StringValue(created.Username)
	if data.SessionTimeoutMs.IsUnknown() {
		data.SessionTimeoutMs = types.Int64Value(created.SessionTimeoutMs)
	}
	if data.Disabled.IsUnknown() {
		data.Disabled = types.BoolValue(created.Disabled)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.client.WithContext(ctx).GetUser(data.Username.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading user", err.Error())
		return
	}
	data.ID = types.StringValue(u.Username)
	data.Username = types.StringValue(u.Username)
	data.FullName = types.StringValue(u.FullName)
	data.Email = types.StringValue(u.Email)
	// roles: Graylog returns them as a set in arbitrary order; keep the prior
	// state ordering when the sets are equal to avoid phantom reorder diffs.
	var priorRoles []string
	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		_ = data.Roles.ElementsAs(ctx, &priorRoles, false)
	}
	if !sameStringMultiset(priorRoles, u.Roles) {
		roleVals := make([]attr.Value, 0, len(u.Roles))
		for _, rname := range u.Roles {
			roleVals = append(roleVals, types.StringValue(rname))
		}
		data.Roles = types.ListValueMust(types.StringType, roleVals)
	}
	// Optional server-defaulted attributes: only refresh them when they are
	// tracked in state, otherwise a server default (e.g. timezone "UTC")
	// produces a permanent diff against a null config value.
	if !data.Timezone.IsNull() {
		data.Timezone = types.StringValue(u.Timezone)
	}
	// session_timeout_ms is Computed: store what the server reports, 0
	// included (users created by older provider versions without the
	// attribute have 0 stored and cannot log in until it is set).
	data.SessionTimeoutMs = types.Int64Value(u.SessionTimeoutMs)
	// disabled is Computed for the same reason.
	data.Disabled = types.BoolValue(u.Disabled)
	// The password is write-only and never returned by the API; keep the
	// prior state value instead of nulling it, otherwise every plan shows
	// a password change and the user is updated on every apply.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var roles []string
	if !data.Roles.IsNull() && !data.Roles.IsUnknown() {
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	updateTimeout, diags := data.Timeouts.Update(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	c := r.client.WithContext(ctx)
	updated, err := c.UpdateUser(data.Username.ValueString(), &client.User{
		Username:         data.Username.ValueString(),
		FullName:         data.FullName.ValueString(),
		Email:            data.Email.ValueString(),
		Roles:            roles,
		Timezone:         data.Timezone.ValueString(),
		SessionTimeoutMs: data.SessionTimeoutMs.ValueInt64(),
		Disabled:         data.Disabled.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating user", err.Error())
		return
	}
	// The id is Computed (== username) and unknown in the plan; set it
	// explicitly so the apply result contains no unknown values.
	data.ID = types.StringValue(data.Username.ValueString())
	if data.SessionTimeoutMs.IsUnknown() {
		data.SessionTimeoutMs = types.Int64Value(updated.SessionTimeoutMs)
	}
	if data.Disabled.IsUnknown() {
		data.Disabled = types.BoolValue(updated.Disabled)
	}
	// The password goes through its own endpoint and only when it changed:
	// Graylog answers 403 for external users, which would otherwise block
	// every unrelated update (e.g. roles) on a directory-managed account.
	if pw := passwordToSet(data.Password, state.Password); pw != "" {
		if err := c.SetUserPassword(data.Username.ValueString(), pw); err != nil {
			// The profile update above already succeeded; persist it and keep
			// the old password in state so the next plan retries only that.
			data.Password = state.Password
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			resp.Diagnostics.AddError("Error updating user password", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, diags := data.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if err := r.client.WithContext(ctx).DeleteUser(data.Username.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting user", err.Error())
		return
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Импорт по username
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
}

// sameStringMultiset reports whether two string slices contain the same
// elements regardless of order (multiset comparison).
func sameStringMultiset(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

// passwordToSet returns the password Update has to push to Graylog, or ""
// when there is nothing to do: the planned value is null/unknown (no
// password configured) or equal to what is already in state.
func passwordToSet(plan, state types.String) string {
	if plan.IsNull() || plan.IsUnknown() || plan.Equal(state) {
		return ""
	}
	return plan.ValueString()
}

// keepStateWhenUnset plans the prior state value when the attribute is not
// set in the configuration. Unlike UseStateForUnknown it also applies when
// the state is null, so an imported user without a configured password does
// not show "(known after apply)" on every unrelated change.
type keepStateWhenUnset struct{}

func (keepStateWhenUnset) Description(context.Context) string {
	return "Keeps the prior state value when the attribute is not configured."
}

func (m keepStateWhenUnset) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepStateWhenUnset) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.StateValue
	}
}
