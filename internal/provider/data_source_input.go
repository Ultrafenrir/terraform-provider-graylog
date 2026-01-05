package provider

import (
	"context"
	"errors"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type inputDataSource struct{ client *client.Client }

type inputDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Title         types.String `tfsdk:"title"`
	Type          types.String `tfsdk:"type"`
	Global        types.Bool   `tfsdk:"global"`
	Node          types.String `tfsdk:"node"`
	Configuration types.Map    `tfsdk:"configuration"`
}

func NewInputDataSource() datasource.DataSource { return &inputDataSource{} }

func (d *inputDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "graylog_input"
}

func (d *inputDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Graylog input by ID. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Required: true, Description: "The unique identifier of the input"},
			"title": schema.StringAttribute{Computed: true, Description: "The title of the input"},
			"type":  schema.StringAttribute{Computed: true, Description: "The input type"},
			"global": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this input is global (available on all nodes)",
			},
			"node": schema.StringAttribute{
				Computed:    true,
				Description: "Node ID where this input runs (if not global)",
			},
			"configuration": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Input-specific configuration parameters",
			},
		},
	}
}

func (d *inputDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client.Client)
}

func (d *inputDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data inputDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in, err := d.client.GetInput(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError("Input not found", "The input with the specified ID was not found")
			return
		}
		resp.Diagnostics.AddError("Error reading input", err.Error())
		return
	}

	data.Title = types.StringValue(in.Title)
	data.Type = types.StringValue(in.Type)
	data.Global = types.BoolValue(in.Global)
	data.Node = types.StringValue(in.Node)

	// Convert configuration to map
	configMap := make(map[string]types.String)
	for k, v := range in.Configuration {
		if str, ok := v.(string); ok {
			configMap[k] = types.StringValue(str)
		}
	}
	var diags diag.Diagnostics
	data.Configuration, diags = types.MapValueFrom(ctx, types.StringType, configMap)
	resp.Diagnostics.Append(diags...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
