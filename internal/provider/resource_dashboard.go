package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dashboardResource struct{ client *client.Client }

type dashboardModel struct {
	ID          types.String   `tfsdk:"id"`
	Title       types.String   `tfsdk:"title"`
	Description types.String   `tfsdk:"description"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewDashboardResource() resource.Resource { return &dashboardResource{} }

func (r *dashboardResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_dashboard"
}

func (r *dashboardResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a Graylog dashboard (classic dashboards)",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Dashboard ID"},
			"title":       schema.StringAttribute{Required: true, Description: "Dashboard title"},
			"description": schema.StringAttribute{Optional: true, Description: "Dashboard description"},
			"timeouts":    timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *dashboardResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *dashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dashboardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Capability gating: classic dashboards CRUD must be supported by this instance
	caps := r.client.GetCapabilities()
	resp.Diagnostics.Append(ensureFeature(ctx, r.client, caps.ClassicDashboardsCRUD, "classic_dashboards_crud", "try Graylog 7.x or appropriate image")...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateDashboard(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	created, err := r.client.WithContext(ctx).CreateDashboard(&client.Dashboard{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating dashboard", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dashboardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	d, err := r.client.WithContext(ctx).GetDashboard(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dashboard", err.Error())
		return
	}
	data.Title = types.StringValue(d.Title)
	data.Description = types.StringValue(d.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dashboardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Capability gating: classic dashboards CRUD must be supported by this instance
	caps := r.client.GetCapabilities()
	resp.Diagnostics.Append(ensureFeature(ctx, r.client, caps.ClassicDashboardsCRUD, "classic_dashboards_crud", "try Graylog 7.x or appropriate image")...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateDashboard(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	_, err := r.client.WithContext(ctx).UpdateDashboard(data.ID.ValueString(), &client.Dashboard{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating dashboard", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// validateDashboard: title required; description soft length limit (2048 chars warning)
func validateDashboard(m *dashboardModel) (d diag.Diagnostics) {
	if m.Title.IsNull() || m.Title.IsUnknown() || m.Title.ValueString() == "" {
		d.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		if len(m.Description.ValueString()) > 2048 {
			d.AddAttributeWarning(path.Root("description"), "Long description", "Attribute 'description' is longer than 2048 characters; consider shortening it.")
		}
	}
	return
}

func (r *dashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data dashboardModel
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

	if err := r.client.WithContext(ctx).DeleteDashboard(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting dashboard", err.Error())
	}
}

func (r *dashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	raw := req.ID
	if raw == "" {
		resp.Diagnostics.AddError("Empty import ID", "Provide a dashboard ID (UUID) or a title to import by title.")
		return
	}
	isUUID := regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString
	isHex24 := regexp.MustCompile(`(?i)^[0-9a-f]{24}$`).MatchString
	val := strings.TrimSpace(raw)
	if isUUID(val) || isHex24(val) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), val)...) //nolint:errcheck
		return
	}
	const prefix = "title:"
	if strings.HasPrefix(strings.ToLower(val), prefix) {
		val = strings.TrimSpace(val[len(prefix):])
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Client is nil; cannot resolve dashboard by title during import.")
		return
	}
	list, err := r.client.WithContext(ctx).ListDashboards()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list dashboards for import", err.Error())
		return
	}
	matches := make([]client.Dashboard, 0)
	for _, d := range list {
		if d.Title == val {
			matches = append(matches, d)
		}
	}
	if len(matches) == 1 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), matches[0].ID)...) //nolint:errcheck
		return
	}
	if len(matches) == 0 {
		resp.Diagnostics.AddError("Dashboard not found by title", "No dashboard found with exact title: "+val+". Provide a UUID or an exact title.")
		return
	}
	resp.Diagnostics.AddError("Multiple dashboards match title", "Found "+fmt.Sprintf("%d", len(matches))+" dashboards with title '"+val+"'. Please import by UUID.")
}
