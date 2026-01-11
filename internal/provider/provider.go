package provider

import (
	"context"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type graylogProvider struct{}

type graylogProviderModel struct {
	URL   types.String `tfsdk:"url"`
	Token types.String `tfsdk:"token"`
}

func (p *graylogProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "graylog"
}

func (p *graylogProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Attributes: map[string]providerschema.Attribute{
			"url":   providerschema.StringAttribute{Required: true},
			"token": providerschema.StringAttribute{Required: true, Sensitive: true},
		},
	}
}

func (p *graylogProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data graylogProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c := client.New(data.URL.ValueString(), data.Token.ValueString())
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *graylogProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewStreamResource,
		NewInputResource,
		NewIndexSetResource,
		NewPipelineResource,
		NewDashboardResource,
		NewDashboardWidgetResource,
		NewAlertResource,
		NewEventNotificationResource,
		NewLDAPSettingResource,
		NewOutputResource,
		NewRoleResource,
		NewUserResource,
	}
}

func (p *graylogProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewStreamDataSource,
		NewInputDataSource,
		NewIndexSetDataSource,
		NewIndexSetDefaultDataSource,
		NewDashboardDataSource,
		NewEventNotificationDataSource,
		NewUserDataSource,
	}
}

func New() provider.Provider { return &graylogProvider{} }
