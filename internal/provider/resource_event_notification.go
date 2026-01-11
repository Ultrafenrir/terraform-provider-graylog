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

type eventNotificationResource struct{ client *client.Client }

type eventNotificationModel struct {
	ID          types.String   `tfsdk:"id"`
	Title       types.String   `tfsdk:"title"`
	Type        types.String   `tfsdk:"type"`
	Description types.String   `tfsdk:"description"`
	Config      types.String   `tfsdk:"config"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewEventNotificationResource() resource.Resource { return &eventNotificationResource{} }

func (r *eventNotificationResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_event_notification"
}

func (r *eventNotificationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages Graylog Event Notification (email/http/slack/pagerduty).",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Notification ID"},
			"title":       schema.StringAttribute{Required: true, Description: "Title"},
			"type":        schema.StringAttribute{Required: true, Description: "Type (email/http/slack/pagerduty)"},
			"description": schema.StringAttribute{Optional: true, Description: "Description"},
			"config":      schema.StringAttribute{Required: true, Description: "JSON-encoded config object for the notification type."},
			"timeouts":    timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *eventNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *eventNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data eventNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Parse config
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Invalid JSON", err.Error())
		return
	}
	createTimeout, diags := data.Timeouts.Create(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	created, err := r.client.CreateEventNotification(&client.EventNotification{
		Title:       data.Title.ValueString(),
		Type:        data.Type.ValueString(),
		Description: data.Description.ValueString(),
		Config:      cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating event notification", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *eventNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data eventNotificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetEventNotification(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading event notification", err.Error())
		return
	}
	data.Title = types.StringValue(got.Title)
	data.Type = types.StringValue(got.Type)
	data.Description = types.StringValue(got.Description)
	b, _ := json.Marshal(got.Config)
	data.Config = types.StringValue(string(b))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *eventNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data eventNotificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &cfg); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Invalid JSON", err.Error())
		return
	}
	updateTimeout, diags := data.Timeouts.Update(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	_, err := r.client.UpdateEventNotification(data.ID.ValueString(), &client.EventNotification{
		Title:       data.Title.ValueString(),
		Type:        data.Type.ValueString(),
		Description: data.Description.ValueString(),
		Config:      cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating event notification", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *eventNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data eventNotificationModel
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
	if err := r.client.DeleteEventNotification(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting event notification", err.Error())
		return
	}
}

func (r *eventNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
