package provider

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type indexSetResource struct{ client *client.Client }

type indexSetModel struct {
	ID               types.String   `tfsdk:"id"`
	Title            types.String   `tfsdk:"title"`
	Description      types.String   `tfsdk:"description"`
	IndexPrefix      types.String   `tfsdk:"index_prefix"`
	Shards           types.Int64    `tfsdk:"shards"`
	Replicas         types.Int64    `tfsdk:"replicas"`
	IndexAnalyzer    types.String   `tfsdk:"index_analyzer"`
	FieldTypeRefresh types.Int64    `tfsdk:"field_type_refresh_interval"`
	IndexOptMaxSeg   types.Int64    `tfsdk:"index_optimization_max_num_segments"`
	IndexOptDisabled types.Bool     `tfsdk:"index_optimization_disabled"`
	Rotation         *strategyModel `tfsdk:"rotation"`
	Retention        *strategyModel `tfsdk:"retention"`
	Default          types.Bool     `tfsdk:"default"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

type strategyModel struct {
	Class  types.String `tfsdk:"class"`
	Config types.Map    `tfsdk:"config"`
}

func NewIndexSetResource() resource.Resource { return &indexSetResource{} }

func (r *indexSetResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_index_set"
}

func (r *indexSetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     4,
		Description: "Manages a Graylog index set resource. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "The unique identifier of the index set"},
			"title":       schema.StringAttribute{Required: true, Description: "The title of the index set"},
			"description": schema.StringAttribute{Optional: true, Description: "Description of the index set"},
			"index_prefix": schema.StringAttribute{
				Required:    true,
				Description: "Index name prefix (lowercase letters, numbers, dash, underscore)",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z0-9_-]+$`),
						"must contain only lowercase letters, numbers, dashes, and underscores",
					),
				},
			},
			"shards": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of Elasticsearch shards (must be >= 0)",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"replicas": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of Elasticsearch replicas (must be >= 0)",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"index_analyzer":                      schema.StringAttribute{Optional: true, Computed: true, Description: "Elasticsearch analyzer to use (defaults to 'standard')"},
			"field_type_refresh_interval":         schema.Int64Attribute{Optional: true, Computed: true, Description: "Field type refresh interval in milliseconds (defaults to 5000)"},
			"index_optimization_max_num_segments": schema.Int64Attribute{Optional: true, Computed: true, Description: "Max number of segments for index optimization (>=1, defaults to 1)"},
			"index_optimization_disabled":         schema.BoolAttribute{Optional: true, Computed: true, Description: "Disable index optimization (defaults to false)"},
			"default":                             schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether this is the default index set"},
			"timeouts":                            timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
		Blocks: map[string]schema.Block{
			"rotation": schema.SingleNestedBlock{
				Description: "Rotation strategy configuration (Graylog 5.x+)",
				Attributes: map[string]schema.Attribute{
					"class":  schema.StringAttribute{Optional: true, Description: "Fully-qualified rotation strategy class"},
					"config": schema.MapAttribute{Optional: true, ElementType: types.StringType, Description: "Rotation strategy config map (string values)"},
				},
			},
			"retention": schema.SingleNestedBlock{
				Description: "Retention strategy configuration (Graylog 5.x+)",
				Attributes: map[string]schema.Attribute{
					"class":  schema.StringAttribute{Optional: true, Description: "Fully-qualified retention strategy class"},
					"config": schema.MapAttribute{Optional: true, ElementType: types.StringType, Description: "Retention strategy config map (string values)"},
				},
			},
		},
	}
}

// UpgradeState migrates prior state versions to the latest schema version.
// v3 -> v4: migrate legacy flat rotation/retention fields into nested blocks with class+config
// and keep safe defaults to avoid nulls in Graylog 5.x+.
func (r *indexSetResource) UpgradeState(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {

	// Read raw state as a dynamic map to avoid tight coupling
	var raw map[string]any
	diags := req.State.GetAttribute(ctx, path.Empty(), &raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Helper to get string attr from raw map
	getStr := func(k string) string {
		if v, ok := raw[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	// Prepare rotation block if missing
	if _, ok := raw["rotation"]; !ok {
		rotSimple := strings.ToLower(getStr("rotation_strategy"))
		// Choose safe defaults
		rotClass := "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
		rotCfg := map[string]string{
			"type":               "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig",
			"max_docs_per_index": "20000000",
		}
		switch rotSimple {
		case "size", "message_size", "bytes", "size_based":
			// Keep count defaults to avoid API incompatibility unless user explicitly migrates
		case "time", "time_based":
			// Same rationale as above
		}
		raw["rotation"] = map[string]any{
			"class":  rotClass,
			"config": rotCfg,
		}
	}

	// Prepare retention block if missing
	if _, ok := raw["retention"]; !ok {
		retSimple := strings.ToLower(getStr("retention_strategy"))
		retClass := "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
		retCfg := map[string]string{
			"type":                  "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig",
			"max_number_of_indices": "20",
		}
		switch retSimple {
		case "close", "closing":
			// Keep deletion defaults for compatibility
		}
		raw["retention"] = map[string]any{
			"class":  retClass,
			"config": retCfg,
		}
	}

	// Ensure analyzer and refresh/optimization have stable defaults in state to avoid drift
	if _, ok := raw["index_analyzer"]; !ok || raw["index_analyzer"] == "" {
		raw["index_analyzer"] = "standard"
	}
	if _, ok := raw["field_type_refresh_interval"]; !ok {
		raw["field_type_refresh_interval"] = int64(5000)
	}
	if _, ok := raw["index_optimization_max_num_segments"]; !ok {
		raw["index_optimization_max_num_segments"] = int64(1)
	}
	if _, ok := raw["index_optimization_disabled"]; !ok {
		raw["index_optimization_disabled"] = false
	}

	// Remove legacy flat strategy attributes to prevent confusion
	delete(raw, "rotation_strategy")
	delete(raw, "retention_strategy")

	// Write back upgraded state
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Empty(), raw)...)
}

func (r *indexSetResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *indexSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateIndexSet(&data)...)
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

	created, err := r.client.WithContext(ctx).CreateIndexSet(&client.IndexSet{
		Title:                           data.Title.ValueString(),
		Description:                     data.Description.ValueString(),
		IndexPrefix:                     data.IndexPrefix.ValueString(),
		Shards:                          int(data.Shards.ValueInt64()),
		Replicas:                        int(data.Replicas.ValueInt64()),
		IndexAnalyzer:                   data.IndexAnalyzer.ValueString(),
		FieldTypeRefreshInterval:        int(data.FieldTypeRefresh.ValueInt64()),
		IndexOptimizationMaxNumSegments: int(data.IndexOptMaxSeg.ValueInt64()),
		IndexOptimizationDisabled:       data.IndexOptDisabled.ValueBool(),
		RotationStrategyClass:           getStrategyClass(data.Rotation),
		RotationStrategyConfig:          mapFromStringMap(ctx, data.Rotation),
		RetentionStrategyClass:          getStrategyClass(data.Retention),
		RetentionStrategyConfig:         mapFromStringMap(ctx, data.Retention),
		Default:                         data.Default.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating index set", err.Error())
		return
	}

	// Read back from API to get the complete state with all server-populated fields
	is, err := r.client.WithContext(ctx).GetIndexSet(created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading index set after create", err.Error())
		return
	}

	// Set ID
	data.ID = types.StringValue(is.ID)

	// Remember if user specified rotation/retention blocks in plan
	// Check both nil and non-null class to handle framework quirks
	wantRotation := data.Rotation != nil && !data.Rotation.Class.IsNull() && !data.Rotation.Class.IsUnknown()
	wantRetention := data.Retention != nil && !data.Retention.Class.IsNull() && !data.Retention.Class.IsUnknown()

	// Remember planned config keys before applying API response
	var plannedRotationKeys, plannedRetentionKeys []string
	if wantRotation && !data.Rotation.Config.IsNull() {
		for k := range data.Rotation.Config.Elements() {
			plannedRotationKeys = append(plannedRotationKeys, k)
		}
	}
	if wantRetention && !data.Retention.Config.IsNull() {
		for k := range data.Retention.Config.Elements() {
			plannedRetentionKeys = append(plannedRetentionKeys, k)
		}
	}

	// Apply all fields from API response
	applyIndexSetReadState(ctx, &data, is)

	// Don't materialize rotation/retention blocks if user didn't specify them
	if !wantRotation {
		data.Rotation = nil
	} else if len(plannedRotationKeys) > 0 && data.Rotation != nil {
		// Filter config to only include planned keys
		data.Rotation.Config = filterMapKeys(ctx, data.Rotation.Config, plannedRotationKeys)
	}
	if !wantRetention {
		data.Retention = nil
	} else if len(plannedRetentionKeys) > 0 && data.Retention != nil {
		// Filter config to only include planned keys
		data.Retention.Config = filterMapKeys(ctx, data.Retention.Config, plannedRetentionKeys)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Remember whether nested blocks were present in prior state to decide on materialization
	hadRotation := data.Rotation != nil && !data.Rotation.Class.IsNull() && !data.Rotation.Class.IsUnknown()
	hadRetention := data.Retention != nil && !data.Retention.Class.IsNull() && !data.Retention.Class.IsUnknown()

	is, err := r.client.WithContext(ctx).GetIndexSet(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Resource was deleted outside of Terraform
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading index set", err.Error())
		return
	}
	applyIndexSetReadState(ctx, &data, is)

	// Don't materialize rotation/retention blocks if they weren't in prior state
	if !hadRotation {
		data.Rotation = nil
	}
	if !hadRetention {
		data.Retention = nil
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan indexSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state to extract the ID (Computed field not in plan)
	var state indexSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateIndexSet(&plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	_, err := r.client.WithContext(ctx).UpdateIndexSet(state.ID.ValueString(), &client.IndexSet{
		Title:                           plan.Title.ValueString(),
		Description:                     plan.Description.ValueString(),
		IndexPrefix:                     plan.IndexPrefix.ValueString(),
		Shards:                          int(plan.Shards.ValueInt64()),
		Replicas:                        int(plan.Replicas.ValueInt64()),
		IndexAnalyzer:                   plan.IndexAnalyzer.ValueString(),
		FieldTypeRefreshInterval:        int(plan.FieldTypeRefresh.ValueInt64()),
		IndexOptimizationMaxNumSegments: int(plan.IndexOptMaxSeg.ValueInt64()),
		IndexOptimizationDisabled:       plan.IndexOptDisabled.ValueBool(),
		RotationStrategyClass:           getStrategyClass(plan.Rotation),
		RotationStrategyConfig:          mapFromStringMap(ctx, plan.Rotation),
		RetentionStrategyClass:          getStrategyClass(plan.Retention),
		RetentionStrategyConfig:         mapFromStringMap(ctx, plan.Retention),
		Default:                         plan.Default.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating index set", err.Error())
		return
	}

	// Read back from API to get the complete updated state
	is, err := r.client.WithContext(ctx).GetIndexSet(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading index set after update", err.Error())
		return
	}

	// Set ID from state
	plan.ID = state.ID

	// Remember if user specified rotation/retention blocks in plan
	// Check both nil and non-null class to handle framework quirks
	wantRotation := plan.Rotation != nil && !plan.Rotation.Class.IsNull() && !plan.Rotation.Class.IsUnknown()
	wantRetention := plan.Retention != nil && !plan.Retention.Class.IsNull() && !plan.Retention.Class.IsUnknown()

	// Remember planned config keys before applying API response
	var plannedRotationKeys, plannedRetentionKeys []string
	if wantRotation && !plan.Rotation.Config.IsNull() {
		for k := range plan.Rotation.Config.Elements() {
			plannedRotationKeys = append(plannedRotationKeys, k)
		}
	}
	if wantRetention && !plan.Retention.Config.IsNull() {
		for k := range plan.Retention.Config.Elements() {
			plannedRetentionKeys = append(plannedRetentionKeys, k)
		}
	}

	// Apply all fields from API response
	applyIndexSetReadState(ctx, &plan, is)

	// Don't materialize rotation/retention blocks if user didn't specify them
	if !wantRotation {
		plan.Rotation = nil
	} else if len(plannedRotationKeys) > 0 && plan.Rotation != nil {
		// Filter config to only include planned keys
		plan.Rotation.Config = filterMapKeys(ctx, plan.Rotation.Config, plannedRotationKeys)
	}
	if !wantRetention {
		plan.Retention = nil
	} else if len(plannedRetentionKeys) > 0 && plan.Retention != nil {
		// Filter config to only include planned keys
		plan.Retention.Config = filterMapKeys(ctx, plan.Retention.Config, plannedRetentionKeys)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *indexSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data indexSetModel
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

	if err := r.client.WithContext(ctx).DeleteIndexSet(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting index set", err.Error())
	}
}

func (r *indexSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Set ID in state first
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)

	// Fetch from API
	is, err := r.client.WithContext(ctx).GetIndexSet(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing index set", err.Error())
		return
	}

	// Read current state (which now has ID set)
	var data indexSetModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply all fields from API, including rotation/retention blocks
	applyIndexSetReadState(ctx, &data, is)

	// On import, always materialize rotation/retention blocks if present in API
	// (don't apply the "had in prior state" logic used in Read)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ---- Helpers ----

func getStrategyClass(m *strategyModel) string {
	if m == nil || m.Class.IsNull() || m.Class.IsUnknown() {
		return ""
	}
	return m.Class.ValueString()
}

func mapFromStringMap(ctx context.Context, m *strategyModel) map[string]any {
	if m == nil || m.Config.IsNull() || m.Config.IsUnknown() {
		return nil
	}
	tmp := map[string]string{}
	// ignore diagnostics here; they will be surfaced earlier on plan decode
	_ = m.Config.ElementsAs(ctx, &tmp, false)
	out := make(map[string]any, len(tmp))
	for k, v := range tmp {
		out[k] = v
	}
	return out
}

func mapToStringMap(ctx context.Context, in map[string]any) types.Map {
	if len(in) == 0 {
		return types.MapNull(types.StringType)
	}
	flat := map[string]string{}
	for k, v := range in {
		// Skip "type" field that Graylog automatically adds to strategy configs
		// User doesn't configure it, so including it causes "new element appeared" errors
		if k == "type" {
			continue
		}
		flat[k] = toString(v)
	}
	mv, _ := types.MapValueFrom(ctx, types.StringType, flat)
	return mv
}

// applyIndexSetReadState normalizes IndexSet read values and fills the Terraform state model
// with stable defaults to avoid plan drift.
func applyIndexSetReadState(ctx context.Context, data *indexSetModel, is *client.IndexSet) {
	data.Title = types.StringValue(is.Title)
	data.Description = types.StringValue(is.Description)
	data.IndexPrefix = types.StringValue(is.IndexPrefix)
	data.Shards = types.Int64Value(int64(is.Shards))
	data.Replicas = types.Int64Value(int64(is.Replicas))

	// Normalize analyzer and numeric defaults to stable values to eliminate plan drift
	analyzer := is.IndexAnalyzer
	if analyzer == "" {
		analyzer = "standard"
	}
	data.IndexAnalyzer = types.StringValue(analyzer)
	ftri := is.FieldTypeRefreshInterval
	if ftri == 0 {
		ftri = 5000
	}
	data.FieldTypeRefresh = types.Int64Value(int64(ftri))
	maxSeg := is.IndexOptimizationMaxNumSegments
	if maxSeg == 0 {
		maxSeg = 1
	}
	data.IndexOptMaxSeg = types.Int64Value(int64(maxSeg))
	data.IndexOptDisabled = types.BoolValue(is.IndexOptimizationDisabled)
	// Flatten rotation/retention blocks if present
	if is.RotationStrategyClass != "" || len(is.RotationStrategyConfig) > 0 {
		data.Rotation = &strategyModel{
			Class:  types.StringValue(is.RotationStrategyClass),
			Config: mapToStringMap(ctx, is.RotationStrategyConfig),
		}
	}
	if is.RetentionStrategyClass != "" || len(is.RetentionStrategyConfig) > 0 {
		data.Retention = &strategyModel{
			Class:  types.StringValue(is.RetentionStrategyClass),
			Config: mapToStringMap(ctx, is.RetentionStrategyConfig),
		}
	}
	data.Default = types.BoolValue(is.Default)
}

// validateIndexSet performs basic checks for key fields.
func validateIndexSet(m *indexSetModel) (d diag.Diagnostics) {
	if m.Title.IsNull() || m.Title.IsUnknown() || m.Title.ValueString() == "" {
		d.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if m.IndexPrefix.IsNull() || m.IndexPrefix.IsUnknown() || m.IndexPrefix.ValueString() == "" {
		d.AddAttributeError(path.Root("index_prefix"), "Invalid index_prefix", "Attribute 'index_prefix' must be a non-empty string.")
	}
	if !m.IndexOptMaxSeg.IsNull() && !m.IndexOptMaxSeg.IsUnknown() && m.IndexOptMaxSeg.ValueInt64() < 1 {
		d.AddAttributeError(path.Root("index_optimization_max_num_segments"), "Invalid max segments", "'index_optimization_max_num_segments' must be >= 1 when specified.")
	}
	if !m.Shards.IsNull() && !m.Shards.IsUnknown() && m.Shards.ValueInt64() < 0 {
		d.AddAttributeError(path.Root("shards"), "Invalid shards", "'shards' must be >= 0.")
	}
	if !m.Replicas.IsNull() && !m.Replicas.IsUnknown() && m.Replicas.ValueInt64() < 0 {
		d.AddAttributeError(path.Root("replicas"), "Invalid replicas", "'replicas' must be >= 0.")
	}
	return
}

// filterMapKeys returns a new map containing only the specified keys from the original map
func filterMapKeys(ctx context.Context, original types.Map, keysToKeep []string) types.Map {
	if original.IsNull() || original.IsUnknown() {
		return original
	}

	elements := original.Elements()
	if len(elements) == 0 || len(keysToKeep) == 0 {
		return original
	}

	keepSet := make(map[string]bool, len(keysToKeep))
	for _, k := range keysToKeep {
		keepSet[k] = true
	}

	filtered := make(map[string]attr.Value, len(keysToKeep))
	for k, v := range elements {
		if keepSet[k] {
			filtered[k] = v
		}
	}

	result, diags := types.MapValue(types.StringType, filtered)
	if diags.HasError() {
		// If filtering fails, return original
		return original
	}
	return result
}
