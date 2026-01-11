package provider

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dashboardWidgetResource struct{ client *client.Client }

type dashboardWidgetModel struct {
	ID            types.String   `tfsdk:"id"`
	DashboardID   types.String   `tfsdk:"dashboard_id"`
	Type          types.String   `tfsdk:"type"`
	Description   types.String   `tfsdk:"description"`
	CacheTime     types.Int64    `tfsdk:"cache_time"`
	Configuration types.String   `tfsdk:"configuration"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewDashboardWidgetResource() resource.Resource { return &dashboardWidgetResource{} }

func (r *dashboardWidgetResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_dashboard_widget"
}

func (r *dashboardWidgetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a widget on a classic Graylog dashboard.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Widget ID"},
			"dashboard_id":  schema.StringAttribute{Required: true, Description: "Dashboard ID"},
			"type":          schema.StringAttribute{Required: true, Description: "Widget type"},
			"description":   schema.StringAttribute{Optional: true},
			"cache_time":    schema.Int64Attribute{Optional: true},
			"configuration": schema.StringAttribute{Required: true, Description: "JSON-encoded widget configuration"},
			"timeouts":      timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *dashboardWidgetResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *dashboardWidgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dashboardWidgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid JSON", err.Error())
		return
	}
	createTimeout, diags := data.Timeouts.Create(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	created, err := r.client.CreateDashboardWidget(data.DashboardID.ValueString(), &client.DashboardWidget{
		Type:          data.Type.ValueString(),
		Description:   data.Description.ValueString(),
		CacheTime:     int(data.CacheTime.ValueInt64()),
		Configuration: cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating dashboard widget", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardWidgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dashboardWidgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetDashboardWidget(data.DashboardID.ValueString(), data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dashboard widget", err.Error())
		return
	}
	data.Type = types.StringValue(got.Type)
	data.Description = types.StringValue(got.Description)
	if got.CacheTime != 0 {
		data.CacheTime = types.Int64Value(int64(got.CacheTime))
	} else {
		data.CacheTime = types.Int64Null()
	}
	b, _ := json.Marshal(got.Configuration)
	data.Configuration = types.StringValue(string(b))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardWidgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dashboardWidgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid JSON", err.Error())
		return
	}
	updateTimeout, diags := data.Timeouts.Update(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	_, err := r.client.UpdateDashboardWidget(data.DashboardID.ValueString(), data.ID.ValueString(), &client.DashboardWidget{
		Type:          data.Type.ValueString(),
		Description:   data.Description.ValueString(),
		CacheTime:     int(data.CacheTime.ValueInt64()),
		Configuration: cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating dashboard widget", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dashboardWidgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data dashboardWidgetModel
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
	if err := r.client.DeleteDashboardWidget(data.DashboardID.ValueString(), data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting dashboard widget", err.Error())
		return
	}
}

func (r *dashboardWidgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Композитный импорт: {dashboard_id}/{widget_id}
	// Без парсинга — сохраняем в id, а dashboard_id потребует указания вручную при next plan (ограничение простоты)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
