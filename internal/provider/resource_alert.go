package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type alertResource struct{ client *client.Client }

type alertModel struct {
	ID              types.String   `tfsdk:"id"`
	Title           types.String   `tfsdk:"title"`
	Description     types.String   `tfsdk:"description"`
	Priority        types.Int64    `tfsdk:"priority"`
	Alert           types.Bool     `tfsdk:"alert"`
	Config          types.String   `tfsdk:"config"`
	NotificationIDs []types.String `tfsdk:"notification_ids"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

// --- Typed Event Definitions (V1): minimal threshold support ---

// alertSeriesModel represents a single aggregation series entry
type alertSeriesModel struct {
	ID       types.String `tfsdk:"id"`
	Function types.String `tfsdk:"function"`
}

// alertThresholdCondModel represents the threshold condition
type alertThresholdCondModel struct {
	Type  types.String  `tfsdk:"type"`
	Value types.Float64 `tfsdk:"value"`
}

// alertExecutionIntervalModel represents execution schedule interval
type alertExecutionIntervalModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.Int64  `tfsdk:"value"`
	Unit  types.String `tfsdk:"unit"`
}

// alertExecutionModel groups execution settings
type alertExecutionModel struct {
	Interval alertExecutionIntervalModel `tfsdk:"interval"`
}

// alertThresholdBlockModel is a typed block for threshold-based events
type alertThresholdBlockModel struct {
	Query          types.String            `tfsdk:"query"`
	SearchWithinMs types.Int64             `tfsdk:"search_within_ms"`
	ExecuteEveryMs types.Int64             `tfsdk:"execute_every_ms"`
	GraceMs        types.Int64             `tfsdk:"grace_ms"`
	BacklogSize    types.Int64             `tfsdk:"backlog_size"`
	GroupBy        []types.String          `tfsdk:"group_by"`
	Series         []alertSeriesModel      `tfsdk:"series"`
	Threshold      alertThresholdCondModel `tfsdk:"threshold"`
	Execution      alertExecutionModel     `tfsdk:"execution"`
	Filter         alertFilterModel        `tfsdk:"filter"`
}

// alertAggregationBlockModel reuses the same shape as threshold for aggregation events
// (query/group_by/series/threshold/execution) and maps to type=aggregation-v1.
type alertAggregationBlockModel = alertThresholdBlockModel

// alertFilterModel represents shorthand filter block (currently only streams)
type alertFilterModel struct {
	Streams []types.String `tfsdk:"streams"`
}

// Extend main model with typed threshold block (optional)
type alertModelV2 struct {
	alertModel
	Threshold   *alertThresholdBlockModel   `tfsdk:"threshold"`
	Aggregation *alertAggregationBlockModel `tfsdk:"aggregation"`
}

func NewAlertResource() resource.Resource { return &alertResource{} }

func (r *alertResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_alert"
}

func (r *alertResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     6,
		Description: "Manages a Graylog Event Definition (alerts)",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Event Definition ID"},
			"title":       schema.StringAttribute{Required: true, Description: "Title"},
			"description": schema.StringAttribute{Optional: true, Description: "Description"},
			"priority":    schema.Int64Attribute{Optional: true, Description: "Priority (severity)"},
			"alert":       schema.BoolAttribute{Optional: true, Description: "Whether to create alerts"},
			// JSON-encoded free-form object remains as an escape-hatch
			"config":           schema.StringAttribute{Optional: true, Description: "JSON-encoded event configuration (free-form object)."},
			"notification_ids": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Notification IDs to trigger"},
			"timeouts":         timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
		Blocks: map[string]schema.Block{
			// Typed threshold block (minimal viable shape for 'threshold-v1')
			"threshold": schema.SingleNestedBlock{
				Description: "Typed configuration for threshold-based Event Definition (maps to type=threshold-v1)",
				Attributes: map[string]schema.Attribute{
					"query":            schema.StringAttribute{Optional: true, Description: "Graylog query (e.g., level:ERROR)"},
					"search_within_ms": schema.Int64Attribute{Optional: true, Description: "Search window in milliseconds"},
					"execute_every_ms": schema.Int64Attribute{Optional: true, Description: "Execution interval in milliseconds"},
					"grace_ms":         schema.Int64Attribute{Optional: true, Description: "Grace period in milliseconds before triggering notifications (non-negative)"},
					"backlog_size":     schema.Int64Attribute{Optional: true, Description: "Number of backlog messages to collect (non-negative)"},
					"group_by":         schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Group-by fields"},
				},
				Blocks: map[string]schema.Block{
					"filter": schema.SingleNestedBlock{
						Description: "Shorthand filters for the event definition (currently streams only)",
						Attributes: map[string]schema.Attribute{
							"streams": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "List of stream IDs to scope the search to"},
						},
					},
					"series": schema.ListNestedBlock{
						Description: "Series definitions (e.g., count())",
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"id":       schema.StringAttribute{Optional: true, Description: "Series ID"},
								"function": schema.StringAttribute{Required: true, Description: "Aggregation function, e.g., count()"},
							},
						},
					},
					"threshold": schema.SingleNestedBlock{
						Description: "Threshold condition (required if 'threshold' block is used)",
						Attributes: map[string]schema.Attribute{
							// Mark as Optional in schema to avoid provider requiring them when the parent block is omitted.
							// We validate presence at runtime when typed block is provided.
							"type":  schema.StringAttribute{Optional: true, Description: "Threshold type (e.g., more|less)"},
							"value": schema.Float64Attribute{Optional: true, Description: "Threshold value"},
						},
					},
					"execution": schema.SingleNestedBlock{
						Description: "Execution schedule (interval)",
						Blocks: map[string]schema.Block{
							"interval": schema.SingleNestedBlock{
								Attributes: map[string]schema.Attribute{
									"type":  schema.StringAttribute{Optional: true, Description: "Interval type (e.g., interval)"},
									"value": schema.Int64Attribute{Optional: true, Description: "Interval value"},
									"unit":  schema.StringAttribute{Optional: true, Description: "Interval unit (e.g., MINUTES)"},
								},
							},
						},
					},
				},
			},
			// Typed aggregation block (maps to type=aggregation-v1); shape mirrors threshold
			"aggregation": schema.SingleNestedBlock{
				Description: "Typed configuration for aggregation-based Event Definition (maps to type=aggregation-v1)",
				Attributes: map[string]schema.Attribute{
					"query":            schema.StringAttribute{Optional: true, Description: "Graylog query (e.g., level:ERROR)"},
					"search_within_ms": schema.Int64Attribute{Optional: true, Description: "Search window in milliseconds"},
					"execute_every_ms": schema.Int64Attribute{Optional: true, Description: "Execution interval in milliseconds"},
					"grace_ms":         schema.Int64Attribute{Optional: true, Description: "Grace period in milliseconds before triggering notifications (non-negative)"},
					"backlog_size":     schema.Int64Attribute{Optional: true, Description: "Number of backlog messages to collect (non-negative)"},
					"group_by":         schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Group-by fields"},
				},
				Blocks: map[string]schema.Block{
					"filter": schema.SingleNestedBlock{
						Description: "Shorthand filters for the event definition (currently streams only)",
						Attributes: map[string]schema.Attribute{
							"streams": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "List of stream IDs to scope the search to"},
						},
					},
					"series": schema.ListNestedBlock{
						Description: "Series definitions (e.g., count())",
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"id":       schema.StringAttribute{Optional: true, Description: "Series ID"},
								"function": schema.StringAttribute{Required: true, Description: "Aggregation function, e.g., count()"},
							},
						},
					},
					// For simplicity we reuse threshold condition for aggregation scenarios as well
					"threshold": schema.SingleNestedBlock{
						Description: "Threshold condition (optional for aggregation)",
						Attributes: map[string]schema.Attribute{
							"type":  schema.StringAttribute{Optional: true, Description: "Threshold type (e.g., more|less)"},
							"value": schema.Float64Attribute{Optional: true, Description: "Threshold value"},
						},
					},
					"execution": schema.SingleNestedBlock{
						Description: "Execution schedule (interval)",
						Blocks: map[string]schema.Block{
							"interval": schema.SingleNestedBlock{
								Attributes: map[string]schema.Attribute{
									"type":  schema.StringAttribute{Optional: true, Description: "Interval type (e.g., interval)"},
									"value": schema.Int64Attribute{Optional: true, Description: "Interval value"},
									"unit":  schema.StringAttribute{Optional: true, Description: "Interval unit (e.g., MINUTES)"},
								},
							},
						},
					},
				},
			},
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
	var dataV2 alertModelV2
	resp.Diagnostics.Append(req.Plan.Get(ctx, &dataV2)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateAlert(ctx, &dataV2.alertModel)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := dataV2.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Prefer typed threshold block if provided; otherwise use free-form config JSON
	cfg, diags := buildEventConfigFromModel(ctx, &dataV2)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var nids []string
	for _, v := range dataV2.NotificationIDs {
		nids = append(nids, v.ValueString())
	}

	created, err := r.client.WithContext(ctx).CreateEventDefinition(&client.EventDefinition{
		Title:           dataV2.Title.ValueString(),
		Description:     dataV2.Description.ValueString(),
		Priority:        int(dataV2.Priority.ValueInt64()),
		Alert:           dataV2.Alert.ValueBool(),
		Config:          cfg,
		NotificationIDs: nids,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert", err.Error())
		return
	}
	// Keep config in state as canonical JSON for stability
	if s, err := CanonicalizeJSONValue(cfg); err == nil {
		dataV2.Config = types.StringValue(s)
	}
	dataV2.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &dataV2)...)
}

func (r *alertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var dataV2 alertModelV2
	resp.Diagnostics.Append(req.State.Get(ctx, &dataV2)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ed, err := r.client.WithContext(ctx).GetEventDefinition(dataV2.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert", err.Error())
		return
	}
	dataV2.Title = types.StringValue(ed.Title)
	dataV2.Description = types.StringValue(ed.Description)
	dataV2.Priority = types.Int64Value(int64(ed.Priority))
	dataV2.Alert = types.BoolValue(ed.Alert)
	// Pass-through config back to state as JSON
	if b, err := json.Marshal(ed.Config); err == nil {
		// Canonicalize for stable plans
		if s, err2 := CanonicalizeJSONFromString(string(b)); err2 == nil {
			dataV2.Config = types.StringValue(s)
		} else {
			dataV2.Config = types.StringValue(string(b))
		}
	}
	// Notification IDs back
	var tv []types.String
	for _, id := range ed.NotificationIDs {
		tv = append(tv, types.StringValue(id))
	}
	dataV2.NotificationIDs = tv
	// Try to populate typed blocks from config if type matches, but only
	// when the corresponding typed block was present in state/plan to avoid
	// surprising drift after imports that use raw config.
	if dataV2.Aggregation != nil {
		populateAggregationFromConfig(&dataV2, ed.Config)
	}
	if dataV2.Threshold != nil {
		populateThresholdFromConfig(&dataV2, ed.Config)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &dataV2)...)
}

func (r *alertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var dataV2 alertModelV2
	resp.Diagnostics.Append(req.Plan.Get(ctx, &dataV2)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateAlert(ctx, &dataV2.alertModel)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := dataV2.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	cfg, diags2 := buildEventConfigFromModel(ctx, &dataV2)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}
	var nids []string
	for _, v := range dataV2.NotificationIDs {
		nids = append(nids, v.ValueString())
	}

	_, err := r.client.WithContext(ctx).UpdateEventDefinition(dataV2.ID.ValueString(), &client.EventDefinition{
		Title:           dataV2.Title.ValueString(),
		Description:     dataV2.Description.ValueString(),
		Priority:        int(dataV2.Priority.ValueInt64()),
		Alert:           dataV2.Alert.ValueBool(),
		Config:          cfg,
		NotificationIDs: nids,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert", err.Error())
		return
	}
	// Keep config in state as canonical JSON for stability
	if s, err := CanonicalizeJSONValue(cfg); err == nil {
		dataV2.Config = types.StringValue(s)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &dataV2)...)
}

func (r *alertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var dataV2 alertModelV2
	resp.Diagnostics.Append(req.State.Get(ctx, &dataV2)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := dataV2.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	if err := r.client.WithContext(ctx).DeleteEventDefinition(dataV2.ID.ValueString()); err != nil {
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
	// config: if provided, must contain non-empty 'type' (when typed block is not used)
	if !m.Config.IsNull() && !m.Config.IsUnknown() && m.Config.ValueString() != "" {
		var tmp map[string]interface{}
		if err := json.Unmarshal([]byte(m.Config.ValueString()), &tmp); err != nil {
			d.AddAttributeError(path.Root("config"), "Invalid config JSON", err.Error())
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

// --- Helpers for typed threshold marshalling/parsing ---

// buildEventConfigFromModel returns Config map assembled either from typed threshold
// (if present) or parsed from free-form JSON in config attribute.
func buildEventConfigFromModel(ctx context.Context, m *alertModelV2) (map[string]interface{}, diag.Diagnostics) {
	var d diag.Diagnostics
	// Detect presence of typed threshold: consider it present if block exists and has query or threshold.type
	hasThreshold := false
	if m.Threshold != nil {
		if !m.Threshold.Query.IsNull() && !m.Threshold.Query.IsUnknown() && m.Threshold.Query.ValueString() != "" {
			hasThreshold = true
		} else if !m.Threshold.Threshold.Type.IsNull() && !m.Threshold.Threshold.Type.IsUnknown() && m.Threshold.Threshold.Type.ValueString() != "" {
			hasThreshold = true
		}
	}
	// Detect presence of typed aggregation (same heuristics)
	hasAggregation := false
	if m.Aggregation != nil {
		if !m.Aggregation.Query.IsNull() && !m.Aggregation.Query.IsUnknown() && m.Aggregation.Query.ValueString() != "" {
			hasAggregation = true
		} else if !m.Aggregation.Threshold.Type.IsNull() && !m.Aggregation.Threshold.Type.IsUnknown() && m.Aggregation.Threshold.Type.ValueString() != "" {
			hasAggregation = true
		}
	}
	// Mutual exclusivity between typed blocks
	if hasThreshold && hasAggregation {
		d.AddError("Ambiguous typed configuration", "Only one typed block may be set: either 'threshold' or 'aggregation'.")
		return nil, d
	}
	if hasThreshold {
		// Validate required fields when typed threshold block is used
		if m.Threshold == nil || m.Threshold.Threshold.Type.IsNull() || m.Threshold.Threshold.Type.IsUnknown() || m.Threshold.Threshold.Type.ValueString() == "" {
			d.AddAttributeError(path.Root("threshold").AtName("threshold").AtName("type"), "Missing threshold.type", "Attribute 'threshold.type' is required when using the typed 'threshold' block.")
		}
		if m.Threshold == nil || m.Threshold.Threshold.Value.IsNull() || m.Threshold.Threshold.Value.IsUnknown() {
			d.AddAttributeError(path.Root("threshold").AtName("threshold").AtName("value"), "Missing threshold.value", "Attribute 'threshold.value' is required when using the typed 'threshold' block.")
		}
		// Non-negative checks
		if !m.Threshold.GraceMs.IsNull() && !m.Threshold.GraceMs.IsUnknown() && m.Threshold.GraceMs.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("threshold").AtName("grace_ms"), "Invalid grace_ms", "'grace_ms' must be a non-negative integer")
		}
		if !m.Threshold.BacklogSize.IsNull() && !m.Threshold.BacklogSize.IsUnknown() && m.Threshold.BacklogSize.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("threshold").AtName("backlog_size"), "Invalid backlog_size", "'backlog_size' must be a non-negative integer")
		}
		// Validate filter.streams entries
		if len(m.Threshold.Filter.Streams) > 0 {
			for i, s := range m.Threshold.Filter.Streams {
				if s.IsNull() || s.IsUnknown() || strings.TrimSpace(s.ValueString()) == "" {
					d.AddAttributeError(path.Root("threshold").AtName("filter").AtName("streams").AtListIndex(i), "Invalid stream id", "Each 'streams' entry must be a non-empty string")
				}
			}
		}
		if d.HasError() {
			return nil, d
		}
		cfg := map[string]interface{}{
			"type": "threshold-v1",
		}
		if !m.Threshold.Query.IsNull() && !m.Threshold.Query.IsUnknown() {
			cfg["query"] = m.Threshold.Query.ValueString()
		}
		if !m.Threshold.SearchWithinMs.IsNull() && !m.Threshold.SearchWithinMs.IsUnknown() {
			cfg["search_within_ms"] = m.Threshold.SearchWithinMs.ValueInt64()
		}
		if !m.Threshold.ExecuteEveryMs.IsNull() && !m.Threshold.ExecuteEveryMs.IsUnknown() {
			cfg["execute_every_ms"] = m.Threshold.ExecuteEveryMs.ValueInt64()
		}
		if !m.Threshold.GraceMs.IsNull() && !m.Threshold.GraceMs.IsUnknown() {
			cfg["grace_ms"] = m.Threshold.GraceMs.ValueInt64()
		}
		if !m.Threshold.BacklogSize.IsNull() && !m.Threshold.BacklogSize.IsUnknown() {
			cfg["backlog_size"] = m.Threshold.BacklogSize.ValueInt64()
		}
		// group_by
		if len(m.Threshold.GroupBy) > 0 {
			arr := make([]string, 0, len(m.Threshold.GroupBy))
			for _, g := range m.Threshold.GroupBy {
				if !g.IsNull() && !g.IsUnknown() && g.ValueString() != "" {
					arr = append(arr, g.ValueString())
				}
			}
			if len(arr) > 0 {
				cfg["group_by"] = arr
			}
		}
		// filter.streams → streams
		if len(m.Threshold.Filter.Streams) > 0 {
			arr := make([]string, 0, len(m.Threshold.Filter.Streams))
			for _, v := range m.Threshold.Filter.Streams {
				if !v.IsNull() && !v.IsUnknown() && strings.TrimSpace(v.ValueString()) != "" {
					arr = append(arr, v.ValueString())
				}
			}
			if len(arr) > 0 {
				cfg["streams"] = arr
			}
		}
		// series
		if len(m.Threshold.Series) > 0 {
			sers := make([]map[string]interface{}, 0, len(m.Threshold.Series))
			for _, s := range m.Threshold.Series {
				item := map[string]interface{}{}
				if !s.ID.IsNull() && !s.ID.IsUnknown() && s.ID.ValueString() != "" {
					item["id"] = s.ID.ValueString()
				}
				if !s.Function.IsNull() && !s.Function.IsUnknown() && s.Function.ValueString() != "" {
					item["function"] = s.Function.ValueString()
				}
				if len(item) > 0 {
					sers = append(sers, item)
				}
			}
			if len(sers) > 0 {
				cfg["series"] = sers
			}
		}
		// threshold condition
		th := map[string]interface{}{}
		if !m.Threshold.Threshold.Type.IsNull() && !m.Threshold.Threshold.Type.IsUnknown() {
			th["type"] = m.Threshold.Threshold.Type.ValueString()
		}
		if !m.Threshold.Threshold.Value.IsNull() && !m.Threshold.Threshold.Value.IsUnknown() {
			th["value"] = m.Threshold.Threshold.Value.ValueFloat64()
		}
		if len(th) > 0 {
			cfg["threshold"] = th
		}
		// execution.interval
		interval := map[string]interface{}{}
		if !m.Threshold.Execution.Interval.Type.IsNull() && !m.Threshold.Execution.Interval.Type.IsUnknown() && m.Threshold.Execution.Interval.Type.ValueString() != "" {
			interval["type"] = m.Threshold.Execution.Interval.Type.ValueString()
		}
		if !m.Threshold.Execution.Interval.Value.IsNull() && !m.Threshold.Execution.Interval.Value.IsUnknown() && m.Threshold.Execution.Interval.Value.ValueInt64() > 0 {
			interval["value"] = m.Threshold.Execution.Interval.Value.ValueInt64()
		}
		if !m.Threshold.Execution.Interval.Unit.IsNull() && !m.Threshold.Execution.Interval.Unit.IsUnknown() && m.Threshold.Execution.Interval.Unit.ValueString() != "" {
			interval["unit"] = m.Threshold.Execution.Interval.Unit.ValueString()
		}
		if len(interval) > 0 {
			cfg["execution"] = map[string]interface{}{"interval": interval}
		}
		return cfg, d
	}

	if hasAggregation {
		// For aggregation we allow threshold to be optional; if provided, we pass it through
		cfg := map[string]interface{}{
			"type": "aggregation-v1",
		}
		if !m.Aggregation.Query.IsNull() && !m.Aggregation.Query.IsUnknown() {
			cfg["query"] = m.Aggregation.Query.ValueString()
		}
		if !m.Aggregation.SearchWithinMs.IsNull() && !m.Aggregation.SearchWithinMs.IsUnknown() {
			cfg["search_within_ms"] = m.Aggregation.SearchWithinMs.ValueInt64()
		}
		if !m.Aggregation.ExecuteEveryMs.IsNull() && !m.Aggregation.ExecuteEveryMs.IsUnknown() {
			cfg["execute_every_ms"] = m.Aggregation.ExecuteEveryMs.ValueInt64()
		}
		// Non-negative checks
		if !m.Aggregation.GraceMs.IsNull() && !m.Aggregation.GraceMs.IsUnknown() && m.Aggregation.GraceMs.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("aggregation").AtName("grace_ms"), "Invalid grace_ms", "'grace_ms' must be a non-negative integer")
		}
		if !m.Aggregation.BacklogSize.IsNull() && !m.Aggregation.BacklogSize.IsUnknown() && m.Aggregation.BacklogSize.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("aggregation").AtName("backlog_size"), "Invalid backlog_size", "'backlog_size' must be a non-negative integer")
		}
		// Validate filter.streams entries
		if len(m.Aggregation.Filter.Streams) > 0 {
			for i, s := range m.Aggregation.Filter.Streams {
				if s.IsNull() || s.IsUnknown() || strings.TrimSpace(s.ValueString()) == "" {
					d.AddAttributeError(path.Root("aggregation").AtName("filter").AtName("streams").AtListIndex(i), "Invalid stream id", "Each 'streams' entry must be a non-empty string")
				}
			}
		}
		if d.HasError() {
			return nil, d
		}
		if !m.Aggregation.GraceMs.IsNull() && !m.Aggregation.GraceMs.IsUnknown() {
			cfg["grace_ms"] = m.Aggregation.GraceMs.ValueInt64()
		}
		if !m.Aggregation.BacklogSize.IsNull() && !m.Aggregation.BacklogSize.IsUnknown() {
			cfg["backlog_size"] = m.Aggregation.BacklogSize.ValueInt64()
		}
		if len(m.Aggregation.GroupBy) > 0 {
			arr := make([]string, 0, len(m.Aggregation.GroupBy))
			for _, g := range m.Aggregation.GroupBy {
				if !g.IsNull() && !g.IsUnknown() && g.ValueString() != "" {
					arr = append(arr, g.ValueString())
				}
			}
			if len(arr) > 0 {
				cfg["group_by"] = arr
			}
		}
		// filter.streams → streams
		if len(m.Aggregation.Filter.Streams) > 0 {
			arr := make([]string, 0, len(m.Aggregation.Filter.Streams))
			for _, v := range m.Aggregation.Filter.Streams {
				if !v.IsNull() && !v.IsUnknown() && strings.TrimSpace(v.ValueString()) != "" {
					arr = append(arr, v.ValueString())
				}
			}
			if len(arr) > 0 {
				cfg["streams"] = arr
			}
		}
		if len(m.Aggregation.Series) > 0 {
			sers := make([]map[string]interface{}, 0, len(m.Aggregation.Series))
			for _, s := range m.Aggregation.Series {
				item := map[string]interface{}{}
				if !s.ID.IsNull() && !s.ID.IsUnknown() && s.ID.ValueString() != "" {
					item["id"] = s.ID.ValueString()
				}
				if !s.Function.IsNull() && !s.Function.IsUnknown() && s.Function.ValueString() != "" {
					item["function"] = s.Function.ValueString()
				}
				if len(item) > 0 {
					sers = append(sers, item)
				}
			}
			if len(sers) > 0 {
				cfg["series"] = sers
			}
		}
		// optional threshold for aggregation
		if m.Aggregation != nil {
			th := map[string]interface{}{}
			if !m.Aggregation.Threshold.Type.IsNull() && !m.Aggregation.Threshold.Type.IsUnknown() && m.Aggregation.Threshold.Type.ValueString() != "" {
				th["type"] = m.Aggregation.Threshold.Type.ValueString()
			}
			if !m.Aggregation.Threshold.Value.IsNull() && !m.Aggregation.Threshold.Value.IsUnknown() {
				th["value"] = m.Aggregation.Threshold.Value.ValueFloat64()
			}
			if len(th) > 0 {
				cfg["threshold"] = th
			}
		}
		// execution.interval
		interval := map[string]interface{}{}
		if !m.Aggregation.Execution.Interval.Type.IsNull() && !m.Aggregation.Execution.Interval.Type.IsUnknown() && m.Aggregation.Execution.Interval.Type.ValueString() != "" {
			interval["type"] = m.Aggregation.Execution.Interval.Type.ValueString()
		}
		if !m.Aggregation.Execution.Interval.Value.IsNull() && !m.Aggregation.Execution.Interval.Value.IsUnknown() && m.Aggregation.Execution.Interval.Value.ValueInt64() > 0 {
			interval["value"] = m.Aggregation.Execution.Interval.Value.ValueInt64()
		}
		if !m.Aggregation.Execution.Interval.Unit.IsNull() && !m.Aggregation.Execution.Interval.Unit.IsUnknown() && m.Aggregation.Execution.Interval.Unit.ValueString() != "" {
			interval["unit"] = m.Aggregation.Execution.Interval.Unit.ValueString()
		}
		if len(interval) > 0 {
			cfg["execution"] = map[string]interface{}{"interval": interval}
		}
		return cfg, d
	}

	// Fallback to free-form JSON in config
	cfg := map[string]interface{}{}
	if !m.Config.IsNull() && !m.Config.IsUnknown() && m.Config.ValueString() != "" {
		if err := json.Unmarshal([]byte(m.Config.ValueString()), &cfg); err != nil {
			d.AddAttributeError(path.Root("config"), "Invalid config JSON", err.Error())
			return nil, d
		}
	}
	return cfg, d
}

// populateThresholdFromConfig attempts to fill typed threshold block from the raw config map
func populateThresholdFromConfig(m *alertModelV2, cfg map[string]interface{}) {
	if cfg == nil {
		return
	}
	if t, ok := cfg["type"].(string); !ok || t != "threshold-v1" {
		return
	}
	if m.Threshold == nil {
		m.Threshold = &alertThresholdBlockModel{}
	}
	if q, ok := cfg["query"].(string); ok {
		m.Threshold.Query = types.StringValue(q)
	}
	if sw, ok := cfg["search_within_ms"].(float64); ok {
		m.Threshold.SearchWithinMs = types.Int64Value(int64(sw))
	}
	if ex, ok := cfg["execute_every_ms"].(float64); ok {
		m.Threshold.ExecuteEveryMs = types.Int64Value(int64(ex))
	}
	if gr, ok := cfg["grace_ms"].(float64); ok {
		m.Threshold.GraceMs = types.Int64Value(int64(gr))
	}
	if bl, ok := cfg["backlog_size"].(float64); ok {
		m.Threshold.BacklogSize = types.Int64Value(int64(bl))
	}
	if gb, ok := cfg["group_by"].([]interface{}); ok {
		g := make([]types.String, 0, len(gb))
		for _, v := range gb {
			if s, ok := v.(string); ok && s != "" {
				g = append(g, types.StringValue(s))
			}
		}
		m.Threshold.GroupBy = g
	}
	if st, ok := cfg["streams"].([]interface{}); ok {
		out := make([]types.String, 0, len(st))
		for _, v := range st {
			if s, ok := v.(string); ok && s != "" {
				out = append(out, types.StringValue(s))
			}
		}
		m.Threshold.Filter.Streams = out
	}
	if ser, ok := cfg["series"].([]interface{}); ok {
		out := make([]alertSeriesModel, 0, len(ser))
		for _, it := range ser {
			if obj, ok := it.(map[string]interface{}); ok {
				var s alertSeriesModel
				if id, ok := obj["id"].(string); ok {
					s.ID = types.StringValue(id)
				}
				if fn, ok := obj["function"].(string); ok {
					s.Function = types.StringValue(fn)
				}
				out = append(out, s)
			}
		}
		m.Threshold.Series = out
	}
	if th, ok := cfg["threshold"].(map[string]interface{}); ok {
		if tp, ok := th["type"].(string); ok {
			m.Threshold.Threshold.Type = types.StringValue(tp)
		}
		if val, ok := th["value"].(float64); ok {
			m.Threshold.Threshold.Value = types.Float64Value(val)
		}
	}
	if ex, ok := cfg["execution"].(map[string]interface{}); ok {
		if in, ok := ex["interval"].(map[string]interface{}); ok {
			if tp, ok := in["type"].(string); ok {
				m.Threshold.Execution.Interval.Type = types.StringValue(tp)
			}
			if vv, ok := in["value"].(float64); ok {
				m.Threshold.Execution.Interval.Value = types.Int64Value(int64(vv))
			}
			if un, ok := in["unit"].(string); ok {
				m.Threshold.Execution.Interval.Unit = types.StringValue(un)
			}
		}
	}
}

// populateAggregationFromConfig attempts to fill typed aggregation block from the raw config map
func populateAggregationFromConfig(m *alertModelV2, cfg map[string]interface{}) {
	if cfg == nil {
		return
	}
	t, ok := cfg["type"].(string)
	if !ok || t != "aggregation-v1" {
		return
	}
	if m.Aggregation == nil {
		m.Aggregation = &alertAggregationBlockModel{}
	}
	if q, ok := cfg["query"].(string); ok {
		m.Aggregation.Query = types.StringValue(q)
	}
	if sw, ok := cfg["search_within_ms"].(float64); ok {
		m.Aggregation.SearchWithinMs = types.Int64Value(int64(sw))
	}
	if ex, ok := cfg["execute_every_ms"].(float64); ok {
		m.Aggregation.ExecuteEveryMs = types.Int64Value(int64(ex))
	}
	if gr, ok := cfg["grace_ms"].(float64); ok {
		m.Aggregation.GraceMs = types.Int64Value(int64(gr))
	}
	if bl, ok := cfg["backlog_size"].(float64); ok {
		m.Aggregation.BacklogSize = types.Int64Value(int64(bl))
	}
	if gb, ok := cfg["group_by"].([]interface{}); ok {
		g := make([]types.String, 0, len(gb))
		for _, v := range gb {
			if s, ok := v.(string); ok && s != "" {
				g = append(g, types.StringValue(s))
			}
		}
		m.Aggregation.GroupBy = g
	}
	if st, ok := cfg["streams"].([]interface{}); ok {
		out := make([]types.String, 0, len(st))
		for _, v := range st {
			if s, ok := v.(string); ok && s != "" {
				out = append(out, types.StringValue(s))
			}
		}
		m.Aggregation.Filter.Streams = out
	}
	if ser, ok := cfg["series"].([]interface{}); ok {
		out := make([]alertSeriesModel, 0, len(ser))
		for _, it := range ser {
			if obj, ok := it.(map[string]interface{}); ok {
				var s alertSeriesModel
				if id, ok := obj["id"].(string); ok {
					s.ID = types.StringValue(id)
				}
				if fn, ok := obj["function"].(string); ok {
					s.Function = types.StringValue(fn)
				}
				out = append(out, s)
			}
		}
		m.Aggregation.Series = out
	}
	if th, ok := cfg["threshold"].(map[string]interface{}); ok {
		if tp, ok := th["type"].(string); ok {
			m.Aggregation.Threshold.Type = types.StringValue(tp)
		}
		if val, ok := th["value"].(float64); ok {
			m.Aggregation.Threshold.Value = types.Float64Value(val)
		}
	}
	if ex, ok := cfg["execution"].(map[string]interface{}); ok {
		if in, ok := ex["interval"].(map[string]interface{}); ok {
			if tp, ok := in["type"].(string); ok {
				m.Aggregation.Execution.Interval.Type = types.StringValue(tp)
			}
			if vv, ok := in["value"].(float64); ok {
				m.Aggregation.Execution.Interval.Value = types.Int64Value(int64(vv))
			}
			if un, ok := in["unit"].(string); ok {
				m.Aggregation.Execution.Interval.Unit = types.StringValue(un)
			}
		}
	}
}
