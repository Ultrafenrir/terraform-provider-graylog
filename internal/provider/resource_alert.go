package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type alertResource struct{ client *client.Client }

type alertModel struct {
	ID              types.String   `tfsdk:"id"`
	Title           types.String   `tfsdk:"title"`
	Description     types.String   `tfsdk:"description"`
	Priority        types.Int64    `tfsdk:"priority"`
	Alert           types.Bool     `tfsdk:"alert"`
	Config          types.Map      `tfsdk:"config"`
	NotificationIDs []types.String `tfsdk:"notification_ids"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

func NewAlertResource() resource.Resource { return &alertResource{} }

func (r *alertResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_alert"
}

func (r *alertResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     2,
		Description: "Manages a Graylog Event Definition (alerts)",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, Description: "Event Definition ID"},
			"title":            schema.StringAttribute{Required: true, Description: "Title"},
			"description":      schema.StringAttribute{Optional: true, Description: "Description"},
			"priority":         schema.Int64Attribute{Optional: true, Description: "Priority (severity)"},
			"alert":            schema.BoolAttribute{Optional: true, Description: "Whether to create alerts"},
			"config":           schema.MapAttribute{Optional: true, ElementType: types.DynamicType, Description: "Event configuration as free-form map; values may be strings, numbers, booleans, or nested objects (pass-through)."},
			"notification_ids": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Notification IDs to trigger"},
			"timeouts":         timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *alertResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *alertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data alertModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateAlert(ctx, &data)...)
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

	// Convert config map(dynamic) -> map[string]interface{}
	cfg := map[string]interface{}{}
	if !data.Config.IsNull() && !data.Config.IsUnknown() {
		tmp := make(map[string]interface{}, len(data.Config.Elements()))
		resp.Diagnostics.Append(data.Config.ElementsAs(ctx, &tmp, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range tmp {
			cfg[k] = v
		}
	}

	var nids []string
	for _, v := range data.NotificationIDs {
		nids = append(nids, v.ValueString())
	}

	created, err := r.client.CreateEventDefinition(&client.EventDefinition{
		Title:           data.Title.ValueString(),
		Description:     data.Description.ValueString(),
		Priority:        int(data.Priority.ValueInt64()),
		Alert:           data.Alert.ValueBool(),
		Config:          cfg,
		NotificationIDs: nids,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *alertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data alertModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ed, err := r.client.GetEventDefinition(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert", err.Error())
		return
	}
	data.Title = types.StringValue(ed.Title)
	data.Description = types.StringValue(ed.Description)
	data.Priority = types.Int64Value(int64(ed.Priority))
	data.Alert = types.BoolValue(ed.Alert)
	// Pass-through dynamic config
	mv, diag := types.MapValueFrom(ctx, types.DynamicType, ed.Config)
	resp.Diagnostics.Append(diag...)
	data.Config = mv
	// Notification IDs back
	var tv []types.String
	for _, id := range ed.NotificationIDs {
		tv = append(tv, types.StringValue(id))
	}
	data.NotificationIDs = tv
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *alertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data alertModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateAlert(ctx, &data)...)
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

	cfg := map[string]interface{}{}
	if !data.Config.IsNull() && !data.Config.IsUnknown() {
		tmp := make(map[string]interface{}, len(data.Config.Elements()))
		resp.Diagnostics.Append(data.Config.ElementsAs(ctx, &tmp, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range tmp {
			cfg[k] = v
		}
	}
	var nids []string
	for _, v := range data.NotificationIDs {
		nids = append(nids, v.ValueString())
	}

	_, err := r.client.UpdateEventDefinition(data.ID.ValueString(), &client.EventDefinition{
		Title:           data.Title.ValueString(),
		Description:     data.Description.ValueString(),
		Priority:        int(data.Priority.ValueInt64()),
		Alert:           data.Alert.ValueBool(),
		Config:          cfg,
		NotificationIDs: nids,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *alertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data alertModel
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

	if err := r.client.DeleteEventDefinition(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting alert", err.Error())
	}
}

func (r *alertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// toString provides a best-effort string conversion for config map values
func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%g", t)
	default:
		return ""
	}
}

// validateAlert checks fields and config for required keys.
func validateAlert(ctx context.Context, m *alertModel) (d diag.Diagnostics) {
	if m.Title.IsNull() || m.Title.IsUnknown() || m.Title.ValueString() == "" {
		d.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if !m.Priority.IsNull() && !m.Priority.IsUnknown() {
		if m.Priority.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("priority"), "Invalid priority", "Attribute 'priority' must be >= 0.")
		}
	}
	// notification_ids: non-empty strings if provided
	for i, v := range m.NotificationIDs {
		if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
			d.AddAttributeError(path.Root("notification_ids").AtListIndex(i), "Invalid notification id", "Each 'notification_ids' entry must be a non-empty string.")
		}
	}
	// config: if provided, must contain non-empty 'type'
	if !m.Config.IsNull() && !m.Config.IsUnknown() {
		tmp := make(map[string]interface{}, len(m.Config.Elements()))
		if di := m.Config.ElementsAs(ctx, &tmp, false); di.HasError() {
			d.Append(di...)
			return d
		}
		v, ok := tmp["type"]
		if !ok {
			d.AddAttributeError(path.Root("config"), "Missing config.type", "Event 'config' must contain key 'type' with a non-empty string value.")
		} else {
			s, sok := v.(string)
			if !sok || s == "" {
				d.AddAttributeError(path.Root("config").AtName("type"), "Empty config.type", "'config.type' must be a non-empty string.")
			}
		}
	}
	return
}
