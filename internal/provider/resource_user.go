package provider

import (
	"context"
	"errors"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
		Description: "Manages Graylog user (local).",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, Description: "Same as username"},
			"username":           schema.StringAttribute{Required: true, Description: "Username (immutable)"},
			"full_name":          schema.StringAttribute{Optional: true},
			"email":              schema.StringAttribute{Optional: true},
			"roles":              schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"timezone":           schema.StringAttribute{Optional: true},
			"session_timeout_ms": schema.Int64Attribute{Optional: true},
			"disabled":           schema.BoolAttribute{Optional: true},
			"password":           schema.StringAttribute{Optional: true, Sensitive: true, Description: "Write-only; changes will update user's password."},
			"timeouts":           timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
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
	// roles
	roleVals := make([]attr.Value, 0, len(u.Roles))
	for _, rname := range u.Roles {
		roleVals = append(roleVals, types.StringValue(rname))
	}
	data.Roles = types.ListValueMust(types.StringType, roleVals)
	data.Timezone = types.StringValue(u.Timezone)
	if u.SessionTimeoutMs != 0 {
		data.SessionTimeoutMs = types.Int64Value(u.SessionTimeoutMs)
	} else {
		data.SessionTimeoutMs = types.Int64Null()
	}
	data.Disabled = types.BoolValue(u.Disabled)
	// Пароль не читается; оставляем unknown/null
	data.Password = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
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
	_, err := r.client.WithContext(ctx).UpdateUser(data.Username.ValueString(), &client.User{
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
		resp.Diagnostics.AddError("Error updating user", err.Error())
		return
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
