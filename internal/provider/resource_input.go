package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type inputResource struct{ client *client.Client }

var _ resource.ResourceWithUpgradeState = (*inputResource)(nil)

type inputExtractorConverterModel struct {
	Type   types.String `tfsdk:"type"`
	Config types.String `tfsdk:"config"`
}

type inputExtractorModel struct {
	ID              types.String                   `tfsdk:"id"`
	Title           types.String                   `tfsdk:"title"`
	ExtractorType   types.String                   `tfsdk:"extractor_type"`
	SourceField     types.String                   `tfsdk:"source_field"`
	TargetField     types.String                   `tfsdk:"target_field"`
	CursorStrategy  types.String                   `tfsdk:"cursor_strategy"`
	ExtractorConfig types.String                   `tfsdk:"extractor_config"`
	ConditionType   types.String                   `tfsdk:"condition_type"`
	ConditionValue  types.String                   `tfsdk:"condition_value"`
	Order           types.Int64                    `tfsdk:"order"`
	Converters      []inputExtractorConverterModel `tfsdk:"converter"`
}

type inputModel struct {
	ID            types.String          `tfsdk:"id"`
	Title         types.String          `tfsdk:"title"`
	Type          types.String          `tfsdk:"type"`
	Global        types.Bool            `tfsdk:"global"`
	Node          types.String          `tfsdk:"node"`
	Configuration types.String          `tfsdk:"configuration"`
	Extractors    []inputExtractorModel `tfsdk:"extractor"`
	Timeouts      timeouts.Value        `tfsdk:"timeouts"`
}

func NewInputResource() resource.Resource { return &inputResource{} }

func (r *inputResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_input"
}

func (r *inputResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     7,
		Description: "Manages a Graylog input resource. Compatible with Graylog v5, v6, and v7.\n- Supports all Kafka input types and settings via flexible configuration.\n- Supports managing input extractors as structured blocks.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, Description: "The unique identifier of the input"},
			"title": schema.StringAttribute{Required: true, Description: "The title of the input"},
			"type":  schema.StringAttribute{Required: true, Description: "The input type (e.g., org.graylog2.inputs.syslog.udp.SyslogUDPInput)"},
			"global": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether this input is global (available on all nodes). Defaults to false.",
				Default:     booldefault.StaticBool(false),
			},
			"node": schema.StringAttribute{
				Optional:    true,
				Description: "Node ID to run this input on (if not global)",
			},
			// Use JSON-encoded strings to represent free-form objects to satisfy framework limitations.
			"configuration": schema.StringAttribute{
				Optional: true,
				Description: "JSON-encoded object with input configuration (free-form). Marked sensitive because " +
					"many input types embed credentials/secrets in here (e.g. Kafka's \"custom_properties\", which " +
					"Graylog itself flags as sensitive, commonly holds SSL keystore/truststore passwords and SASL " +
					"JAAS config with embedded passwords).",
				Sensitive: true,
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
		Blocks: map[string]schema.Block{
			"extractor": schema.ListNestedBlock{
				Description: "Input extractor. Extractors transform/enrich message fields as they arrive on this input.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":             schema.StringAttribute{Computed: true, Description: "Extractor ID"},
						"title":          schema.StringAttribute{Required: true, Description: "Extractor title"},
						"extractor_type": schema.StringAttribute{Required: true, Description: "Extractor type, e.g. \"regex\", \"grok\", \"substring\", \"split_and_index\", \"copy_input\", \"regex_replace\", \"json\", \"lookup_table\" (availability depends on Graylog version/plugins)"},
						"source_field":   schema.StringAttribute{Required: true, Description: "Message field to read from"},
						"target_field":   schema.StringAttribute{Optional: true, Description: "Message field to write the extracted value to"},
						"cursor_strategy": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "How the source field's raw value is treated after extraction: \"copy\" (leave untouched) or \"cut\" (remove the matched part). Defaults to \"copy\".",
							Validators: []validator.String{
								stringvalidator.OneOf("copy", "cut"),
							},
							Default: stringdefault.StaticString("copy"),
						},
						"extractor_config": schema.StringAttribute{
							Optional:    true,
							Description: "JSON-encoded extractor-type-specific configuration (e.g. {\"regex_value\": \"...\"} for regex, {\"grok_pattern\": \"...\"} for grok).",
						},
						"condition_type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "When to run this extractor: \"none\" (always), \"string\" (source field contains condition_value), or \"regex\" (source field matches condition_value). Defaults to \"none\".",
							Validators: []validator.String{
								stringvalidator.OneOf("none", "string", "regex"),
							},
							Default: stringdefault.StaticString("none"),
						},
						"condition_value": schema.StringAttribute{
							Optional:    true,
							Description: "Condition string/regex; required when condition_type is not \"none\".",
						},
						"order": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Extractor execution order. If omitted, Graylog assigns the next available position.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"converter": schema.ListNestedBlock{
							Description: "Converters applied to the extracted value, in order.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"type":   schema.StringAttribute{Required: true, Description: "Converter type, e.g. \"numeric\", \"lowercase\", \"uppercase\", \"hash\", \"date\", \"csv\", \"tokenizer\", \"ip_anonymizer\", \"splitandcount\", \"syslog_pri\""},
									"config": schema.StringAttribute{Optional: true, Description: "JSON-encoded converter-specific configuration."},
								},
							},
						},
					},
				},
			},
		},
	}
}

// inputModelV6 mirrors the pre-v7 schema shape, where extractors were stored as a single
// JSON-encoded string attribute instead of structured "extractor" blocks.
type inputModelV6 struct {
	ID            types.String   `tfsdk:"id"`
	Title         types.String   `tfsdk:"title"`
	Type          types.String   `tfsdk:"type"`
	Global        types.Bool     `tfsdk:"global"`
	Node          types.String   `tfsdk:"node"`
	Configuration types.String   `tfsdk:"configuration"`
	Extractors    types.String   `tfsdk:"extractors"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

// UpgradeState declares the state upgrade path from schema version 6 (the last version where
// "extractors" was a JSON-encoded string) to version 7 (structured "extractor" blocks).
func (r *inputResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	priorSchemaV6 := schema.Schema{
		Version: 6,
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"title":         schema.StringAttribute{Required: true},
			"type":          schema.StringAttribute{Required: true},
			"global":        schema.BoolAttribute{Optional: true, Computed: true},
			"node":          schema.StringAttribute{Optional: true},
			"configuration": schema.StringAttribute{Optional: true},
			"extractors":    schema.StringAttribute{Optional: true},
			"timeouts":      timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
	return map[int64]resource.StateUpgrader{
		6: {
			PriorSchema:   &priorSchemaV6,
			StateUpgrader: r.upgradeStateV6ToV7,
		},
	}
}

func (r *inputResource) upgradeStateV6ToV7(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var prior inputModelV6
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newData := inputModel{
		ID:            prior.ID,
		Title:         prior.Title,
		Type:          prior.Type,
		Global:        prior.Global,
		Node:          prior.Node,
		Configuration: prior.Configuration,
		Timeouts:      prior.Timeouts,
	}
	if newData.Global.IsNull() || newData.Global.IsUnknown() {
		newData.Global = types.BoolValue(false)
	}

	if !prior.Extractors.IsNull() && !prior.Extractors.IsUnknown() && prior.Extractors.ValueString() != "" {
		// The old free-form extractor JSON was always overwritten on Read with Graylog's own
		// extractor object shape, so it can be parsed directly as client.Extractor.
		var rawExtractors []client.Extractor
		if err := json.Unmarshal([]byte(prior.Extractors.ValueString()), &rawExtractors); err != nil {
			resp.Diagnostics.AddError(
				"Unable to Upgrade Extractors",
				"The previous 'extractors' JSON could not be parsed while migrating to structured 'extractor' blocks: "+err.Error()+
					". Remove the extractors from state manually (terraform state rm) and re-import or re-create them after upgrading.",
			)
			return
		}
		newData.Extractors = make([]inputExtractorModel, 0, len(rawExtractors))
		for _, ex := range rawExtractors {
			m, d := extractorClientToModel(ex)
			resp.Diagnostics.Append(d...)
			newData.Extractors = append(newData.Extractors, m)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// validateInput performs runtime validation of the input model and appends diagnostics on issues.
func validateInput(ctx context.Context, data *inputModel) (diags diag.Diagnostics) {
	if data.Title.IsNull() || data.Title.IsUnknown() || data.Title.ValueString() == "" {
		diags.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	if data.Type.IsNull() || data.Type.IsUnknown() || data.Type.ValueString() == "" {
		diags.AddAttributeError(path.Root("type"), "Invalid type", "Attribute 'type' must be a non-empty string with a Graylog input class name.")
	}
	// Cross-field: global/node
	if !data.Global.IsUnknown() && !data.Global.IsNull() && !data.Global.ValueBool() {
		if data.Node.IsNull() || data.Node.IsUnknown() || data.Node.ValueString() == "" {
			diags.AddAttributeError(path.Root("node"), "Missing node for non-global input", "When 'global' is false, 'node' must be specified with a non-empty node ID.")
		}
	}
	// Extractors: basic cross-field validation
	for i, ex := range data.Extractors {
		if ex.Title.IsNull() || ex.Title.IsUnknown() || ex.Title.ValueString() == "" {
			diags.AddAttributeError(path.Root("extractor").AtListIndex(i).AtName("title"), "Invalid extractor title", "Each extractor must have a non-empty 'title'.")
		}
		if ex.ExtractorType.IsNull() || ex.ExtractorType.IsUnknown() || ex.ExtractorType.ValueString() == "" {
			diags.AddAttributeError(path.Root("extractor").AtListIndex(i).AtName("extractor_type"), "Invalid extractor_type", "Each extractor must have a non-empty 'extractor_type'.")
		}
		if ex.SourceField.IsNull() || ex.SourceField.IsUnknown() || ex.SourceField.ValueString() == "" {
			diags.AddAttributeError(path.Root("extractor").AtListIndex(i).AtName("source_field"), "Invalid source_field", "Each extractor must have a non-empty 'source_field'.")
		}
		condType := ex.ConditionType.ValueString()
		if condType == "string" || condType == "regex" {
			if ex.ConditionValue.IsNull() || ex.ConditionValue.IsUnknown() || ex.ConditionValue.ValueString() == "" {
				diags.AddAttributeError(path.Root("extractor").AtListIndex(i).AtName("condition_value"), "Missing condition_value", "When 'condition_type' is 'string' or 'regex', 'condition_value' must be set.")
			}
		}
	}
	return
}

func (r *inputResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

// canonicalizeExtractorModel rewrites extractor_config and converter config JSON strings into a
// stable canonical form so identical desired/actual extractors produce the same identity key
// (see extractorIdentityKey), and so re-reading state after apply doesn't show a spurious diff.
func canonicalizeExtractorModel(m *inputExtractorModel) (diags diag.Diagnostics) {
	if !m.ExtractorConfig.IsNull() && !m.ExtractorConfig.IsUnknown() && m.ExtractorConfig.ValueString() != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(m.ExtractorConfig.ValueString()), &cfg); err != nil {
			diags.AddAttributeError(path.Root("extractor").AtName("extractor_config"), "Invalid extractor_config JSON", err.Error())
			return diags
		}
		if canon, err := CanonicalizeJSONValue(cfg); err == nil {
			m.ExtractorConfig = types.StringValue(canon)
		}
	}
	for i := range m.Converters {
		c := &m.Converters[i]
		if c.Config.IsNull() || c.Config.IsUnknown() || c.Config.ValueString() == "" {
			continue
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(c.Config.ValueString()), &cfg); err != nil {
			diags.AddAttributeError(path.Root("extractor").AtName("converter"), "Invalid converter config JSON", err.Error())
			return diags
		}
		if canon, err := CanonicalizeJSONValue(cfg); err == nil {
			c.Config = types.StringValue(canon)
		}
	}
	return diags
}

// extractorIdentityKey builds a stable key from every user-meaningful field of an extractor
// (everything except the server-assigned id). Two extractors with the same key are considered
// the same extractor for reconciliation purposes during Update.
func extractorIdentityKey(m inputExtractorModel) string {
	var b strings.Builder
	b.WriteString(m.Title.ValueString())
	b.WriteString("|")
	b.WriteString(m.ExtractorType.ValueString())
	b.WriteString("|")
	b.WriteString(m.SourceField.ValueString())
	b.WriteString("|")
	b.WriteString(m.TargetField.ValueString())
	b.WriteString("|")
	b.WriteString(m.CursorStrategy.ValueString())
	b.WriteString("|")
	b.WriteString(m.ConditionType.ValueString())
	b.WriteString("|")
	b.WriteString(m.ConditionValue.ValueString())
	b.WriteString("|")
	b.WriteString(m.ExtractorConfig.ValueString())
	for _, c := range m.Converters {
		b.WriteString("|conv:")
		b.WriteString(c.Type.ValueString())
		b.WriteString(":")
		b.WriteString(c.Config.ValueString())
	}
	return b.String()
}

// extractorModelToClient converts a plan-side extractor model into the API payload.
func extractorModelToClient(m inputExtractorModel) (*client.Extractor, diag.Diagnostics) {
	var diags diag.Diagnostics
	ex := &client.Extractor{
		Title:          m.Title.ValueString(),
		ExtractorType:  m.ExtractorType.ValueString(),
		SourceField:    m.SourceField.ValueString(),
		TargetField:    m.TargetField.ValueString(),
		CursorStrategy: m.CursorStrategy.ValueString(),
		ConditionType:  m.ConditionType.ValueString(),
		ConditionValue: m.ConditionValue.ValueString(),
		// Graylog requires an explicit JSON array even when no converters are configured.
		Converters: make([]client.ExtractorConverter, 0),
	}
	if !m.Order.IsNull() && !m.Order.IsUnknown() {
		ex.Order = int(m.Order.ValueInt64())
	}
	if !m.ExtractorConfig.IsNull() && !m.ExtractorConfig.IsUnknown() && m.ExtractorConfig.ValueString() != "" {
		cfg := make(map[string]interface{})
		if err := json.Unmarshal([]byte(m.ExtractorConfig.ValueString()), &cfg); err != nil {
			diags.AddAttributeError(path.Root("extractor").AtName("extractor_config"), "Invalid extractor_config JSON", err.Error())
			return nil, diags
		}
		ex.ExtractorConfig = cfg
	}
	for _, c := range m.Converters {
		conv := client.ExtractorConverter{Type: c.Type.ValueString()}
		if !c.Config.IsNull() && !c.Config.IsUnknown() && c.Config.ValueString() != "" {
			cfg := make(map[string]interface{})
			if err := json.Unmarshal([]byte(c.Config.ValueString()), &cfg); err != nil {
				diags.AddAttributeError(path.Root("extractor").AtName("converter"), "Invalid converter config JSON", err.Error())
				return nil, diags
			}
			conv.Config = cfg
		}
		ex.Converters = append(ex.Converters, conv)
	}
	return ex, diags
}

// extractorClientToModel converts an API extractor object (from Create/List) into the Terraform
// model. Fields that are Optional-only (not Computed) are mapped to Null rather than an empty
// string when absent, so state matches what an unconfigured attribute would plan as.
func extractorClientToModel(ex client.Extractor) (inputExtractorModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	m := inputExtractorModel{
		ID:            types.StringValue(ex.ID),
		Title:         types.StringValue(ex.Title),
		ExtractorType: types.StringValue(ex.ExtractorType),
		SourceField:   types.StringValue(ex.SourceField),
		Order:         types.Int64Value(int64(ex.Order)),
	}
	if ex.TargetField != "" {
		m.TargetField = types.StringValue(ex.TargetField)
	} else {
		m.TargetField = types.StringNull()
	}
	if ex.CursorStrategy != "" {
		m.CursorStrategy = types.StringValue(ex.CursorStrategy)
	} else {
		m.CursorStrategy = types.StringValue("copy")
	}
	if ex.ConditionType != "" {
		m.ConditionType = types.StringValue(ex.ConditionType)
	} else {
		m.ConditionType = types.StringValue("none")
	}
	if ex.ConditionValue != "" {
		m.ConditionValue = types.StringValue(ex.ConditionValue)
	} else {
		m.ConditionValue = types.StringNull()
	}
	if len(ex.ExtractorConfig) > 0 {
		canon, err := CanonicalizeJSONValue(ex.ExtractorConfig)
		if err != nil {
			diags.AddError("Error encoding extractor_config", err.Error())
			return m, diags
		}
		m.ExtractorConfig = types.StringValue(canon)
	} else {
		m.ExtractorConfig = types.StringNull()
	}
	m.Converters = make([]inputExtractorConverterModel, 0, len(ex.Converters))
	for _, c := range ex.Converters {
		cm := inputExtractorConverterModel{Type: types.StringValue(c.Type)}
		if len(c.Config) > 0 {
			canon, err := CanonicalizeJSONValue(c.Config)
			if err != nil {
				diags.AddError("Error encoding converter config", err.Error())
				return m, diags
			}
			cm.Config = types.StringValue(canon)
		} else {
			cm.Config = types.StringNull()
		}
		m.Converters = append(m.Converters, cm)
	}
	return m, diags
}

func (r *inputResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateInput(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Convert configuration (JSON string) into map[string]interface{}
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() && data.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &config); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid configuration JSON", err.Error())
			return
		}
		// Canonicalize configuration JSON to avoid noisy diffs
		if canon, err := CanonicalizeJSONValue(config); err == nil {
			data.Configuration = types.StringValue(canon)
		}
	}

	in := &client.Input{
		Title:         data.Title.ValueString(),
		Type:          data.Type.ValueString(),
		Global:        data.Global.ValueBool(),
		Node:          data.Node.ValueString(),
		Configuration: config,
	}
	created, err := r.client.WithContext(ctx).CreateInput(in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating input", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	// Note: deliberately not syncing data.Global from created.Global here. Graylog's create
	// response doesn't reliably echo "global" back, and since global is Optional+Computed with a
	// static Default, its planned value is always already known (either user-configured or the
	// resolved default) by the time we get here — Terraform requires Create to return that exact
	// same known value. Any genuine server-side drift is picked up on the next Read/refresh.

	// From this point on the input exists in Graylog. Canonicalize desired extractor configs up
	// front, then create them one by one, always persisting whatever succeeded so far into state
	// (even on a later error) — this prevents orphaning the input or corrupting state on a
	// partial failure.
	for i := range data.Extractors {
		resp.Diagnostics.Append(canonicalizeExtractorModel(&data.Extractors[i])...)
	}
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	for i := range data.Extractors {
		payload, d := extractorModelToClient(data.Extractors[i])
		resp.Diagnostics.Append(d...)
		if d.HasError() {
			resolveUnknownExtractorState(&data.Extractors[i])
			continue
		}
		createdEx, cerr := r.client.WithContext(ctx).CreateInputExtractor(data.ID.ValueString(), payload)
		if cerr != nil {
			resp.Diagnostics.AddError("Error creating input extractor", cerr.Error())
			resolveUnknownExtractorState(&data.Extractors[i])
			continue
		}
		newModel, d2 := extractorClientToModel(*createdEx)
		resp.Diagnostics.Append(d2...)
		data.Extractors[i] = newModel
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// resolveUnknownExtractorState prevents a secondary "invalid result object" error when Graylog
// rejects one extractor after the parent input has already been created. Computed values may be
// null in state, but must not remain unknown after Apply.
func resolveUnknownExtractorState(m *inputExtractorModel) {
	if m.ID.IsUnknown() {
		m.ID = types.StringNull()
	}
	if m.Order.IsUnknown() {
		m.Order = types.Int64Null()
	}
}

func (r *inputResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, err := r.client.WithContext(ctx).GetInput(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Resource was deleted outside of Terraform
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading input", err.Error())
		return
	}
	data.Title = types.StringValue(in.Title)
	data.Type = types.StringValue(in.Type)
	data.Global = types.BoolValue(in.Global)
	if in.Node != "" {
		data.Node = types.StringValue(in.Node)
	} else {
		data.Node = types.StringNull()
	}

	// Set configuration back as canonical JSON string
	if in.Configuration != nil {
		if canon, err := CanonicalizeJSONValue(in.Configuration); err == nil {
			data.Configuration = types.StringValue(canon)
		} else if b, err2 := json.Marshal(in.Configuration); err2 == nil {
			data.Configuration = types.StringValue(string(b))
		}
	}

	// Read extractors
	if exList, err := r.client.WithContext(ctx).ListInputExtractors(data.ID.ValueString()); err == nil {
		out := make([]inputExtractorModel, 0, len(exList))
		for _, ex := range exList {
			m, d := extractorClientToModel(ex)
			resp.Diagnostics.Append(d...)
			out = append(out, m)
		}
		data.Extractors = out
	} else {
		resp.Diagnostics.AddWarning("Unable to read input extractors", err.Error())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// id is Computed (not Optional) with no plan modifier, so it is Unknown/empty in the plan —
	// it must come from prior state instead. Without this, every Update sent requests to
	// "/api/system/inputs/" (empty ID), which Graylog rejects with 405.
	var state inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = state.ID

	// Runtime validation
	resp.Diagnostics.Append(validateInput(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Convert configuration JSON to map
	config := make(map[string]interface{})
	if !data.Configuration.IsNull() && !data.Configuration.IsUnknown() && data.Configuration.ValueString() != "" {
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &config); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid configuration JSON", err.Error())
			return
		}
		if canon, err := CanonicalizeJSONValue(config); err == nil {
			data.Configuration = types.StringValue(canon)
		}
	}

	in := &client.Input{
		Title:         data.Title.ValueString(),
		Type:          data.Type.ValueString(),
		Global:        data.Global.ValueBool(),
		Node:          data.Node.ValueString(),
		Configuration: config,
	}
	_, err := r.client.WithContext(ctx).UpdateInput(data.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating input", err.Error())
		return
	}
	// Note: deliberately not syncing data.Global from the update response — see the matching
	// comment in Create(); Graylog's response isn't a reliable source for this field, and the
	// plan's known value must be returned unchanged for Terraform's consistency check to pass.

	// Reconcile extractors by identity: only delete extractors that are no longer desired and
	// only create extractors that don't already exist — unchanged extractors are left alone
	// instead of being destroyed and recreated on every apply.
	for i := range data.Extractors {
		resp.Diagnostics.Append(canonicalizeExtractorModel(&data.Extractors[i])...)
	}
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	existing, err := r.client.WithContext(ctx).ListInputExtractors(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing input extractors", err.Error())
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	existingModels := make([]inputExtractorModel, 0, len(existing))
	for _, ex := range existing {
		m, d := extractorClientToModel(ex)
		resp.Diagnostics.Append(d...)
		existingModels = append(existingModels, m)
	}

	exByKey := make(map[string]inputExtractorModel, len(existingModels))
	for _, m := range existingModels {
		exByKey[extractorIdentityKey(m)] = m
	}
	desiredKeys := make(map[string]struct{}, len(data.Extractors))
	for _, m := range data.Extractors {
		desiredKeys[extractorIdentityKey(m)] = struct{}{}
	}

	// Delete extractors that are no longer desired.
	for _, m := range existingModels {
		if _, ok := desiredKeys[extractorIdentityKey(m)]; ok {
			continue
		}
		if id := m.ID.ValueString(); id != "" {
			if derr := r.client.WithContext(ctx).DeleteInputExtractor(data.ID.ValueString(), id); derr != nil {
				resp.Diagnostics.AddError("Error deleting input extractor", derr.Error())
			}
		}
	}

	// Create extractors that don't already exist; reuse the existing id/order for matches.
	for i := range data.Extractors {
		key := extractorIdentityKey(data.Extractors[i])
		if existingMatch, ok := exByKey[key]; ok {
			data.Extractors[i].ID = existingMatch.ID
			if data.Extractors[i].Order.IsNull() || data.Extractors[i].Order.IsUnknown() {
				data.Extractors[i].Order = existingMatch.Order
			}
			continue
		}
		payload, d := extractorModelToClient(data.Extractors[i])
		resp.Diagnostics.Append(d...)
		if d.HasError() {
			continue
		}
		createdEx, cerr := r.client.WithContext(ctx).CreateInputExtractor(data.ID.ValueString(), payload)
		if cerr != nil {
			resp.Diagnostics.AddError("Error creating input extractor", cerr.Error())
			continue
		}
		newModel, d2 := extractorClientToModel(*createdEx)
		resp.Diagnostics.Append(d2...)
		data.Extractors[i] = newModel
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *inputResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data inputModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	deleteTimeout, diags := data.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	if err := r.client.WithContext(ctx).DeleteInput(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting input", err.Error())
	}
}

func (r *inputResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
