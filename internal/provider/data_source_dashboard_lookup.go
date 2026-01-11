package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dashboardDataSource struct{ client *client.Client }

type dashboardDSModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
}

func NewDashboardDataSource() datasource.DataSource { return &dashboardDataSource{} }

func (d *dashboardDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_dashboard"
}

func (d *dashboardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lookup Graylog dashboard by ID or title (classic dashboards). Set exactly one of id or title.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Optional: true, Description: "Dashboard ID"},
			"title":       schema.StringAttribute{Optional: true, Description: "Dashboard title (exact match)"},
			"description": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *dashboardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *dashboardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data dashboardDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		got, err := d.client.GetDashboard(data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("id"), "Dashboard not found", err.Error())
			return
		}
		data.Title = types.StringValue(got.Title)
		data.Description = types.StringValue(got.Description)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	if !data.Title.IsNull() && data.Title.ValueString() != "" {
		list, err := d.client.ListDashboards()
		if err != nil {
			resp.Diagnostics.AddError("Lookup failed", err.Error())
			return
		}
		title := data.Title.ValueString()
		for i := range list {
			if list[i].Title == title {
				data.ID = types.StringValue(list[i].ID)
				data.Description = types.StringValue(list[i].Description)
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
		resp.Diagnostics.AddAttributeError(path.Root("title"), "Dashboard not found", "No dashboard found with the specified title")
		return
	}
	resp.Diagnostics.AddError("Missing lookup key", "Provide either 'id' or 'title' to lookup a dashboard")
}
