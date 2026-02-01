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

// LDAP settings is a singleton resource in Graylog
type ldapSettingResource struct{ client *client.Client }

type ldapSettingModel struct {
	ID                   types.String   `tfsdk:"id"`
	Enabled              types.Bool     `tfsdk:"enabled"`
	SystemUsername       types.String   `tfsdk:"system_username"`
	SystemPassword       types.String   `tfsdk:"system_password"`
	LDAPURI              types.String   `tfsdk:"ldap_uri"`
	SearchBase           types.String   `tfsdk:"search_base"`
	SearchPattern        types.String   `tfsdk:"search_pattern"`
	UserUniqueIDAttr     types.String   `tfsdk:"user_unique_id_attribute"`
	GroupSearchBase      types.String   `tfsdk:"group_search_base"`
	GroupSearchPattern   types.String   `tfsdk:"group_search_pattern"`
	DefaultGroup         types.String   `tfsdk:"default_group"`
	UseStartTLS          types.Bool     `tfsdk:"use_start_tls"`
	TrustAllCertificates types.Bool     `tfsdk:"trust_all_certificates"`
	ActiveDirectory      types.Bool     `tfsdk:"active_directory"`
	DisplayNameAttr      types.String   `tfsdk:"display_name_attribute"`
	EmailAttr            types.String   `tfsdk:"email_attribute"`
	Timeouts             timeouts.Value `tfsdk:"timeouts"`
}

func NewLDAPSettingResource() resource.Resource { return &ldapSettingResource{} }

func (r *ldapSettingResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_ldap_setting"
}

func (r *ldapSettingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Graylog global LDAP settings (singleton).",
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Computed: true, Description: "Fixed ID for singleton (always 'ldap')"},
			"enabled":                  schema.BoolAttribute{Optional: true, Description: "Enable LDAP authentication"},
			"system_username":          schema.StringAttribute{Optional: true, Description: "Bind DN / system username"},
			"system_password":          schema.StringAttribute{Optional: true, Sensitive: true, Description: "Bind password"},
			"ldap_uri":                 schema.StringAttribute{Optional: true, Description: "LDAP URI"},
			"search_base":              schema.StringAttribute{Optional: true, Description: "User search base DN"},
			"search_pattern":           schema.StringAttribute{Optional: true, Description: "User search pattern"},
			"user_unique_id_attribute": schema.StringAttribute{Optional: true, Description: "Attribute for unique user ID"},
			"group_search_base":        schema.StringAttribute{Optional: true, Description: "Group search base DN"},
			"group_search_pattern":     schema.StringAttribute{Optional: true, Description: "Group search pattern"},
			"default_group":            schema.StringAttribute{Optional: true, Description: "Default Graylog group"},
			"use_start_tls":            schema.BoolAttribute{Optional: true, Description: "Use STARTTLS"},
			"trust_all_certificates":   schema.BoolAttribute{Optional: true, Description: "Trust all certificates"},
			"active_directory":         schema.BoolAttribute{Optional: true, Description: "Active Directory mode"},
			"display_name_attribute":   schema.StringAttribute{Optional: true, Description: "Display name attribute"},
			"email_attribute":          schema.StringAttribute{Optional: true, Description: "Email attribute"},
			"timeouts":                 timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *ldapSettingResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *ldapSettingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ldapSettingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Map to client struct and update
	_, err := r.client.WithContext(ctx).UpdateLDAPSettings(&client.LDAPSettings{
		Enabled:               data.Enabled.ValueBool(),
		SystemUsername:        data.SystemUsername.ValueString(),
		SystemPassword:        data.SystemPassword.ValueString(),
		LDAPURI:               data.LDAPURI.ValueString(),
		SearchBase:            data.SearchBase.ValueString(),
		SearchPattern:         data.SearchPattern.ValueString(),
		UserUniqueIDAttribute: data.UserUniqueIDAttr.ValueString(),
		GroupSearchBase:       data.GroupSearchBase.ValueString(),
		GroupSearchPattern:    data.GroupSearchPattern.ValueString(),
		DefaultGroup:          data.DefaultGroup.ValueString(),
		UseStartTLS:           data.UseStartTLS.ValueBool(),
		TrustAllCertificates:  data.TrustAllCertificates.ValueBool(),
		ActiveDirectory:       data.ActiveDirectory.ValueBool(),
		DisplayNameAttribute:  data.DisplayNameAttr.ValueString(),
		EmailAttribute:        data.EmailAttr.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating LDAP settings", err.Error())
		return
	}
	data.ID = types.StringValue("ldap")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ldapSettingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ldapSettingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.WithContext(ctx).GetLDAPSettings()
	if err != nil {
		resp.Diagnostics.AddError("Error reading LDAP settings", err.Error())
		return
	}
	data.ID = types.StringValue("ldap")
	data.Enabled = types.BoolValue(s.Enabled)
	data.SystemUsername = types.StringValue(s.SystemUsername)
	// Пароли не возвращаются API — не перетирать значение в стейте, оставляем как есть
	data.LDAPURI = types.StringValue(s.LDAPURI)
	data.SearchBase = types.StringValue(s.SearchBase)
	data.SearchPattern = types.StringValue(s.SearchPattern)
	data.UserUniqueIDAttr = types.StringValue(s.UserUniqueIDAttribute)
	data.GroupSearchBase = types.StringValue(s.GroupSearchBase)
	data.GroupSearchPattern = types.StringValue(s.GroupSearchPattern)
	data.DefaultGroup = types.StringValue(s.DefaultGroup)
	data.UseStartTLS = types.BoolValue(s.UseStartTLS)
	data.TrustAllCertificates = types.BoolValue(s.TrustAllCertificates)
	data.ActiveDirectory = types.BoolValue(s.ActiveDirectory)
	data.DisplayNameAttr = types.StringValue(s.DisplayNameAttribute)
	data.EmailAttr = types.StringValue(s.EmailAttribute)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ldapSettingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ldapSettingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.WithContext(ctx).UpdateLDAPSettings(&client.LDAPSettings{
		Enabled:               data.Enabled.ValueBool(),
		SystemUsername:        data.SystemUsername.ValueString(),
		SystemPassword:        data.SystemPassword.ValueString(),
		LDAPURI:               data.LDAPURI.ValueString(),
		SearchBase:            data.SearchBase.ValueString(),
		SearchPattern:         data.SearchPattern.ValueString(),
		UserUniqueIDAttribute: data.UserUniqueIDAttr.ValueString(),
		GroupSearchBase:       data.GroupSearchBase.ValueString(),
		GroupSearchPattern:    data.GroupSearchPattern.ValueString(),
		DefaultGroup:          data.DefaultGroup.ValueString(),
		UseStartTLS:           data.UseStartTLS.ValueBool(),
		TrustAllCertificates:  data.TrustAllCertificates.ValueBool(),
		ActiveDirectory:       data.ActiveDirectory.ValueBool(),
		DisplayNameAttribute:  data.DisplayNameAttr.ValueString(),
		EmailAttribute:        data.EmailAttr.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating LDAP settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ldapSettingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Best-effort: disable LDAP and clear bind creds
	_ = req // not needed to read state
	_, err := r.client.WithContext(ctx).UpdateLDAPSettings(&client.LDAPSettings{Enabled: false})
	if err != nil {
		// Не считаем критичным, просто удаляем из стейта
	}
}

func (r *ldapSettingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Accept any ID and set constant id
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "ldap")...)
}
