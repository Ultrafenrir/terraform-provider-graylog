package provider

import (
	"context"
	"encoding/json"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type eventNotificationDataSource struct{ client *client.Client }

type eventNotificationDSModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Config      types.String `tfsdk:"config"`
}

func NewEventNotificationDataSource() datasource.DataSource { return &eventNotificationDataSource{} }

func (d *eventNotificationDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_event_notification"
}

func (d *eventNotificationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lookup Graylog Event Notification by ID or title (set exactly one of id or title).",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Optional: true, Description: "Event Notification ID"},
			"title":       schema.StringAttribute{Optional: true, Description: "Event Notification title (exact match)"},
			"type":        schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
			"config":      schema.StringAttribute{Computed: true, Description: "JSON-encoded config"},
		},
	}
}

func (d *eventNotificationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *eventNotificationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data eventNotificationDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var n *client.EventNotification
	var err error
	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		n, err = d.client.WithContext(ctx).GetEventNotification(data.ID.ValueString())
	} else if !data.Title.IsNull() && data.Title.ValueString() != "" {
		list, lerr := d.client.WithContext(ctx).ListEventNotifications()
		if lerr != nil {
			err = lerr
		} else {
			title := data.Title.ValueString()
			for i := range list {
				if list[i].Title == title {
					n = &list[i]
					break
				}
			}
			if n == nil {
				resp.Diagnostics.AddAttributeError(path.Root("title"), "Event Notification not found", "No notification found with the specified title")
				return
			}
		}
	} else {
		resp.Diagnostics.AddError("Missing lookup key", "Provide either 'id' or 'title' to lookup an event notification")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Lookup failed", err.Error())
		return
	}
	data.Title = types.StringValue(n.Title)
	data.Type = types.StringValue(n.Type)
	data.Description = types.StringValue(n.Description)
	if n.Config != nil {
		b, _ := json.Marshal(n.Config)
		data.Config = types.StringValue(string(b))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
