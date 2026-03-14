package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ldapGroupMembersDataSource struct{}

type ldapGroupMembersModel struct {
	ID        types.String `tfsdk:"id"`
	URL       types.String `tfsdk:"url"`
	StartTLS  types.Bool   `tfsdk:"starttls"`
	Insecure  types.Bool   `tfsdk:"insecure_skip_verify"`
	BindDN    types.String `tfsdk:"bind_dn"`
	Password  types.String `tfsdk:"bind_password"`
	BaseDN    types.String `tfsdk:"base_dn"`
	GroupName types.String `tfsdk:"group_name"`

	// Attribute mapping (defaults sensible for groupOfNames)
	GroupFilter     types.String `tfsdk:"group_filter"`      // e.g., (cn=%s)
	MemberAttr      types.String `tfsdk:"member_attr"`       // member
	UserFilter      types.String `tfsdk:"user_filter"`       // (objectClass=inetOrgPerson)
	UserIDAttr      types.String `tfsdk:"user_id_attr"`      // uid or sAMAccountName
	EmailAttr       types.String `tfsdk:"email_attr"`        // mail
	DisplayNameAttr types.String `tfsdk:"display_name_attr"` // cn or displayName

	Members []ldapMember `tfsdk:"members"`
}

type ldapMember struct {
	Username    types.String `tfsdk:"username"`
	DN          types.String `tfsdk:"dn"`
	Email       types.String `tfsdk:"email"`
	DisplayName types.String `tfsdk:"display_name"`
}

func NewLDAPGroupMembersDataSource() datasource.DataSource { return &ldapGroupMembersDataSource{} }

func (d *ldapGroupMembersDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_ldap_group_members"
}

func (d *ldapGroupMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads members of an LDAP group by name (safe V1, read-only).",
		Attributes: map[string]schema.Attribute{
			"id":                   schema.StringAttribute{Computed: true, Description: "Synthetic ID: <group_name>@<base_dn>"},
			"url":                  schema.StringAttribute{Required: true, Description: "LDAP URL, e.g., ldap://host:389 or ldaps://host:636"},
			"starttls":             schema.BoolAttribute{Optional: true, Description: "Use StartTLS on LDAP connection"},
			"insecure_skip_verify": schema.BoolAttribute{Optional: true, Description: "Skip TLS verification (dev/test only)"},
			"bind_dn":              schema.StringAttribute{Required: true, Description: "Bind DN"},
			"bind_password":        schema.StringAttribute{Required: true, Sensitive: true, Description: "Bind password"},
			"base_dn":              schema.StringAttribute{Required: true, Description: "Base DN for search"},
			"group_name":           schema.StringAttribute{Required: true, Description: "Group common name (cn) to lookup"},

			"group_filter":      schema.StringAttribute{Optional: true, Description: "Group search filter with %s placeholder for group name (default: (cn=%s))"},
			"member_attr":       schema.StringAttribute{Optional: true, Description: "Attribute holding member DN (default: member)"},
			"user_filter":       schema.StringAttribute{Optional: true, Description: "Filter to apply to user objects (default: (objectClass=inetOrgPerson))"},
			"user_id_attr":      schema.StringAttribute{Optional: true, Description: "User ID attribute to emit as username (default: uid)"},
			"email_attr":        schema.StringAttribute{Optional: true, Description: "Email attribute (default: mail)"},
			"display_name_attr": schema.StringAttribute{Optional: true, Description: "Display name attribute (default: cn)"},

			"members": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Resolved group members",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"username":     schema.StringAttribute{Computed: true},
						"dn":           schema.StringAttribute{Computed: true},
						"email":        schema.StringAttribute{Computed: true},
						"display_name": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ldapGroupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ldapGroupMembersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	url := data.URL.ValueString()
	if url == "" {
		resp.Diagnostics.AddError("missing url", "'url' is required")
		return
	}
	conn, err := ldap.DialURL(url)
	if err != nil {
		resp.Diagnostics.AddError("ldap connect failed", err.Error())
		return
	}
	defer conn.Close()
	if !data.StartTLS.IsNull() && data.StartTLS.ValueBool() {
		tlsConf := &tls.Config{InsecureSkipVerify: !data.Insecure.IsNull() && data.Insecure.ValueBool()}
		if err := conn.StartTLS(tlsConf); err != nil {
			resp.Diagnostics.AddError("ldap starttls failed", err.Error())
			return
		}
	}
	if err := conn.Bind(data.BindDN.ValueString(), data.Password.ValueString()); err != nil {
		resp.Diagnostics.AddError("ldap bind failed", err.Error())
		return
	}
	groupFilter := firstNonEmpty(getString(data.GroupFilter), "(cn=%s)")
	memberAttr := firstNonEmpty(getString(data.MemberAttr), "member")
	userFilter := firstNonEmpty(getString(data.UserFilter), "(objectClass=inetOrgPerson)")
	userIDAttr := firstNonEmpty(getString(data.UserIDAttr), "uid")
	emailAttr := firstNonEmpty(getString(data.EmailAttr), "mail")
	dnAttr := "dn"
	displayNameAttr := firstNonEmpty(getString(data.DisplayNameAttr), "cn")

	// Find group by name
	gf := fmt.Sprintf(groupFilter, ldap.EscapeFilter(data.GroupName.ValueString()))
	gs := ldap.NewSearchRequest(
		data.BaseDN.ValueString(),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&%s)", gf),
		[]string{memberAttr},
		nil,
	)
	gres, err := conn.Search(gs)
	if err != nil {
		resp.Diagnostics.AddError("ldap search group failed", err.Error())
		return
	}
	if len(gres.Entries) == 0 {
		// empty group
		data.ID = types.StringValue(data.GroupName.ValueString() + "@" + data.BaseDN.ValueString())
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	members := []ldapMember{}
	for _, e := range gres.Entries {
		for _, mdn := range e.GetAttributeValues(memberAttr) {
			// Lookup user by DN; validate user matches filter
			us := ldap.NewSearchRequest(
				mdn,
				ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
				fmt.Sprintf("(&%s)", userFilter),
				[]string{userIDAttr, emailAttr, displayNameAttr},
				nil,
			)
			ures, err := conn.Search(us)
			if err != nil {
				// skip broken member but continue
				continue
			}
			if len(ures.Entries) == 0 {
				continue
			}
			ue := ures.Entries[0]
			m := ldapMember{
				Username:    types.StringValue(firstNonEmpty(ue.GetAttributeValue(userIDAttr), strings.TrimPrefix(strings.ToLower(mdn), dnAttr+"="))),
				DN:          types.StringValue(mdn),
				Email:       types.StringValue(ue.GetAttributeValue(emailAttr)),
				DisplayName: types.StringValue(ue.GetAttributeValue(displayNameAttr)),
			}
			members = append(members, m)
		}
	}
	data.Members = members
	data.ID = types.StringValue(data.GroupName.ValueString() + "@" + data.BaseDN.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
