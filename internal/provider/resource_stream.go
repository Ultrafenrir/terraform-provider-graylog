package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type streamResource struct{ client *client.Client }

var _ resource.ResourceWithUpgradeState = (*streamResource)(nil)

type streamRuleModel struct {
	ID          types.String `tfsdk:"id"`
	Field       types.String `tfsdk:"field"`
	Type        types.Int64  `tfsdk:"type"`
	Value       types.String `tfsdk:"value"`
	Inverted    types.Bool   `tfsdk:"inverted"`
	Description types.String `tfsdk:"description"`
}

type streamModel struct {
	ID                       types.String      `tfsdk:"id"`
	Title                    types.String      `tfsdk:"title"`
	Description              types.String      `tfsdk:"description"`
	Disabled                 types.Bool        `tfsdk:"disabled"`
	IndexSetID               types.String      `tfsdk:"index_set_id"`
	MatchingType             types.String      `tfsdk:"matching_type"`
	RemoveMatchesFromDefault types.Bool        `tfsdk:"remove_matches_from_default_stream"`
	Rules                    []streamRuleModel `tfsdk:"rule"`
	Timeouts                 timeouts.Value    `tfsdk:"timeouts"`
}

func NewStreamResource() resource.Resource { return &streamResource{} }

func (r *streamResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_stream"
}

func (r *streamResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     4,
		Description: "Manages a Graylog stream resource. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, Description: "The unique identifier of the stream"},
			"title":        schema.StringAttribute{Required: true, Description: "The title of the stream"},
			"description":  schema.StringAttribute{Optional: true, Description: "Description of the stream"},
			"disabled":     schema.BoolAttribute{Optional: true, Description: "Whether the stream is disabled"},
			"index_set_id": schema.StringAttribute{Optional: true, Description: "The index set ID to use for this stream"},
			"matching_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "How stream rules are combined to match a message: \"AND\" (all rules must match) or \"OR\" (any rule matches). Defaults to \"AND\".",
				Validators: []validator.String{
					stringvalidator.OneOf("AND", "OR"),
				},
				Default: stringdefault.StaticString("AND"),
			},
			"remove_matches_from_default_stream": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "When true, messages matching this stream are removed from the default stream",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				Description: "Stream routing rules",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, Description: "Stream rule ID"},
						"field":       schema.StringAttribute{Required: true, Description: "Field name to match"},
						"type":        schema.Int64Attribute{Required: true, Description: "Rule type (Graylog enum as integer)"},
						"value":       schema.StringAttribute{Required: true, Description: "Value to match"},
						"inverted":    schema.BoolAttribute{Optional: true, Description: "Invert rule condition"},
						"description": schema.StringAttribute{Optional: true, Description: "Rule description"},
					},
				},
			},
		},
	}
}

// UpgradeState declares the state upgrade path for every schema version prior to the
// current one (4). All schema changes so far (v2->v3 added remove_matches_from_default_stream,
// v3->v4 added matching_type) have been purely additive Optional+Computed attributes, so a
// single version-agnostic upgrader handles all of them: it re-decodes the raw prior state
// against the current schema type (missing attributes decode as null) and fills in defaults
// for any null computed attributes.
//
// PriorSchema is intentionally left unset on each StateUpgrader: since the prior schema
// shape for old versions isn't tracked separately, upgradeState works directly from
// req.RawState instead of req.State.
func (r *streamResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	upgrader := resource.StateUpgrader{StateUpgrader: r.upgradeState}
	return map[int64]resource.StateUpgrader{
		0: upgrader,
		1: upgrader,
		2: upgrader,
		3: upgrader,
	}
}

func (r *streamResource) upgradeState(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.RawState == nil {
		resp.Diagnostics.AddError("Unable to Upgrade Resource State", "No prior state was provided to upgrade.")
		return
	}

	currentType := resp.State.Schema.Type().TerraformType(ctx)
	rawValue, err := req.RawState.UnmarshalWithOpts(currentType, tfprotov6.UnmarshalOpts{
		ValueFromJSONOpts: tftypes.ValueFromJSONOpts{IgnoreUndefinedAttributes: true},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Previously Saved State for UpgradeResourceState",
			"There was an error reading the saved resource state using the current resource schema: "+err.Error(),
		)
		return
	}
	resp.State.Raw = rawValue

	var data streamModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Ensure remove_matches_from_default_stream has a value (added in v3)
	if data.RemoveMatchesFromDefault.IsNull() || data.RemoveMatchesFromDefault.IsUnknown() {
		data.RemoveMatchesFromDefault = types.BoolValue(false)
	}
	// Ensure matching_type has a value (added in v4); "AND" matches Graylog's own default
	if data.MatchingType.IsNull() || data.MatchingType.IsUnknown() || data.MatchingType.ValueString() == "" {
		data.MatchingType = types.StringValue("AND")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *streamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation
	resp.Diagnostics.Append(validateStream(&data)...)
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

	// If remove_matches_from_default_stream is not set, use false as default
	removeMatches := false
	if !data.RemoveMatchesFromDefault.IsNull() && !data.RemoveMatchesFromDefault.IsUnknown() {
		removeMatches = data.RemoveMatchesFromDefault.ValueBool()
	}

	// If matching_type is not set, use "AND" as default (API default)
	matchingType := "AND"
	if !data.MatchingType.IsNull() && !data.MatchingType.IsUnknown() {
		matchingType = data.MatchingType.ValueString()
	}

	created, err := r.client.WithContext(ctx).CreateStream(&client.Stream{
		Title:                          data.Title.ValueString(),
		Description:                    data.Description.ValueString(),
		Disabled:                       data.Disabled.ValueBool(),
		IndexSetID:                     data.IndexSetID.ValueString(),
		MatchingType:                   matchingType,
		RemoveMatchesFromDefaultStream: removeMatches,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating stream", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	// Read back actual values from API to ensure all computed fields are populated
	data.RemoveMatchesFromDefault = types.BoolValue(created.RemoveMatchesFromDefaultStream)
	if created.MatchingType != "" {
		data.MatchingType = types.StringValue(created.MatchingType)
	} else {
		data.MatchingType = types.StringValue(matchingType)
	}
	// Only set disabled if it was specified in config
	if !data.Disabled.IsNull() && !data.Disabled.IsUnknown() {
		data.Disabled = types.BoolValue(created.Disabled)
	}
	// Create rules if provided via dedicated API. The stream itself already exists at this
	// point, so on a per-rule failure we keep going (best effort) and always persist whatever
	// succeeded via resp.State.Set below, instead of returning early and losing track of the
	// stream (which would either orphan it or cause a duplicate on the next apply).
	for i, rr := range data.Rules {
		rule := &client.StreamRule{
			Field:       rr.Field.ValueString(),
			Type:        int(rr.Type.ValueInt64()),
			Value:       rr.Value.ValueString(),
			Inverted:    rr.Inverted.ValueBool(),
			Description: rr.Description.ValueString(),
		}
		cr, err := r.client.WithContext(ctx).CreateStreamRule(data.ID.ValueString(), rule)
		if err != nil {
			resp.Diagnostics.AddError("Error creating stream rule", err.Error())
			continue
		}
		// Update IDs in state slice
		if cr != nil && cr.ID != "" {
			// write back into slice element by index (range copy fix)
			data.Rules[i].ID = types.StringValue(cr.ID)
		}
	}
	// Fallback: if some rule IDs are still unknown/empty, fetch from API and map them back
	needMap := false
	for _, rr := range data.Rules {
		if rr.ID.IsNull() || rr.ID.IsUnknown() || rr.ID.ValueString() == "" {
			needMap = true
			break
		}
	}
	if needMap {
		if rules, err := r.client.WithContext(ctx).ListStreamRules(data.ID.ValueString()); err == nil {
			for i, rr := range data.Rules {
				if !rr.ID.IsNull() && !rr.ID.IsUnknown() && rr.ID.ValueString() != "" {
					continue
				}
				// try to find a matching rule by properties
				for _, ar := range rules {
					if ar.Field == rr.Field.ValueString() && ar.Value == rr.Value.ValueString() && ar.Type == int(rr.Type.ValueInt64()) && ar.Inverted == rr.Inverted.ValueBool() {
						data.Rules[i].ID = types.StringValue(ar.ID)
						break
					}
				}
			}
		} else {
			resp.Diagnostics.AddWarning("Unable to map stream rule IDs", err.Error())
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data streamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Remember if disabled was in prior state
	hadDisabled := !data.Disabled.IsNull() && !data.Disabled.IsUnknown()

	s, err := r.client.WithContext(ctx).GetStream(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Resource was deleted outside of Terraform
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading stream", err.Error())
		return
	}
	data.Title = types.StringValue(s.Title)
	// description and index_set_id are Optional (not Computed): mapping an empty server value to
	// StringValue("") instead of Null would permanently disagree with an unconfigured (null)
	// planned value and force a diff on every single plan.
	if s.Description != "" {
		data.Description = types.StringValue(s.Description)
	} else {
		data.Description = types.StringNull()
	}
	// Only materialize disabled if it was in prior state
	if hadDisabled {
		data.Disabled = types.BoolValue(s.Disabled)
	}
	if s.IndexSetID != "" {
		data.IndexSetID = types.StringValue(s.IndexSetID)
	} else {
		data.IndexSetID = types.StringNull()
	}
	data.RemoveMatchesFromDefault = types.BoolValue(s.RemoveMatchesFromDefaultStream)
	if s.MatchingType != "" {
		data.MatchingType = types.StringValue(s.MatchingType)
	} else {
		data.MatchingType = types.StringValue("AND")
	}

	// Remember which optional fields were present in prior state for rules
	priorRulesMap := make(map[string]streamRuleModel) // key: field+type+value
	for _, pr := range data.Rules {
		key := pr.Field.ValueString() + "|" + pr.Type.String() + "|" + pr.Value.ValueString()
		priorRulesMap[key] = pr
	}

	// Read stream rules via API
	if rules, err := r.client.WithContext(ctx).ListStreamRules(data.ID.ValueString()); err == nil {
		out := make([]streamRuleModel, 0, len(rules))
		for _, rrule := range rules {
			key := rrule.Field + "|" + fmt.Sprintf("%d", rrule.Type) + "|" + rrule.Value
			priorRule, hadPrior := priorRulesMap[key]

			newRule := streamRuleModel{
				ID:    types.StringValue(rrule.ID),
				Field: types.StringValue(rrule.Field),
				Type:  types.Int64Value(int64(rrule.Type)),
				Value: types.StringValue(rrule.Value),
			}

			// Only materialize inverted if it was in prior state
			if hadPrior && !priorRule.Inverted.IsNull() && !priorRule.Inverted.IsUnknown() {
				newRule.Inverted = types.BoolValue(rrule.Inverted)
			}
			// Only materialize description if it was in prior state
			if hadPrior && !priorRule.Description.IsNull() && !priorRule.Description.IsUnknown() {
				newRule.Description = types.StringValue(rrule.Description)
			}

			out = append(out, newRule)
		}
		data.Rules = out
	} else {
		resp.Diagnostics.AddWarning("Unable to read stream rules", err.Error())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan streamModel
	var state streamModel
	// Use ID from state for updates (often absent in plan)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Runtime validation (based on plan)
	resp.Diagnostics.Append(validateStream(&plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply timeout
	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	streamID := state.ID.ValueString()

	// If remove_matches_from_default_stream is not set in plan, preserve from state
	removeMatches := state.RemoveMatchesFromDefault.ValueBool()
	if !plan.RemoveMatchesFromDefault.IsNull() && !plan.RemoveMatchesFromDefault.IsUnknown() {
		removeMatches = plan.RemoveMatchesFromDefault.ValueBool()
	}

	// If matching_type is not set in plan, preserve from state (falling back to "AND")
	matchingType := state.MatchingType.ValueString()
	if matchingType == "" {
		matchingType = "AND"
	}
	if !plan.MatchingType.IsNull() && !plan.MatchingType.IsUnknown() {
		matchingType = plan.MatchingType.ValueString()
	}

	_, err := r.client.WithContext(ctx).UpdateStream(streamID, &client.Stream{
		Title:                          plan.Title.ValueString(),
		Description:                    plan.Description.ValueString(),
		Disabled:                       plan.Disabled.ValueBool(),
		IndexSetID:                     plan.IndexSetID.ValueString(),
		MatchingType:                   matchingType,
		RemoveMatchesFromDefaultStream: removeMatches,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating stream", err.Error())
		return
	}
	// The stream itself was already updated above, so from here on always persist state (even
	// on a later error) — update state: keep ID from state; other fields come from the plan.
	plan.ID = types.StringValue(streamID)
	// Ensure remove_matches_from_default_stream is set to the actual value used
	plan.RemoveMatchesFromDefault = types.BoolValue(removeMatches)
	// Ensure matching_type is set to the actual value used
	plan.MatchingType = types.StringValue(matchingType)

	// Diff-aware sync of rules: delete extra, create missing; keep matching ones
	// Build maps by stable key
	existing, err := r.client.WithContext(ctx).ListStreamRules(streamID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing stream rules", err.Error())
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	type ruleKey string
	makeKey := func(f string, t int, v string, inv bool, desc string) ruleKey {
		return ruleKey(fmt.Sprintf("%s|%d|%s|%t|%s", f, t, v, inv, desc))
	}
	exByKey := make(map[ruleKey]string)
	for _, ex := range existing {
		k := makeKey(ex.Field, ex.Type, ex.Value, ex.Inverted, ex.Description)
		exByKey[k] = ex.ID
	}
	desiredKeys := make(map[ruleKey]struct{})
	for _, rr := range plan.Rules {
		k := makeKey(rr.Field.ValueString(), int(rr.Type.ValueInt64()), rr.Value.ValueString(), rr.Inverted.ValueBool(), rr.Description.ValueString())
		desiredKeys[k] = struct{}{}
	}
	// Delete rules that are not desired
	for _, ex := range existing {
		k := makeKey(ex.Field, ex.Type, ex.Value, ex.Inverted, ex.Description)
		if _, ok := desiredKeys[k]; !ok {
			if ex.ID != "" {
				if derr := r.client.WithContext(ctx).DeleteStreamRule(streamID, ex.ID); derr != nil {
					resp.Diagnostics.AddError("Error deleting stream rule", derr.Error())
				}
			}
		}
	}
	// Create rules that are missing. Best effort: keep going on a per-rule failure so we don't
	// abandon the remaining rules, and always persist whatever succeeded below.
	for i, rr := range plan.Rules {
		k := makeKey(rr.Field.ValueString(), int(rr.Type.ValueInt64()), rr.Value.ValueString(), rr.Inverted.ValueBool(), rr.Description.ValueString())
		if _, ok := exByKey[k]; ok {
			// Already present — if ID known in state, keep it; otherwise fill from map
			if plan.Rules[i].ID.IsNull() || plan.Rules[i].ID.IsUnknown() || plan.Rules[i].ID.ValueString() == "" {
				if id := exByKey[k]; id != "" {
					plan.Rules[i].ID = types.StringValue(id)
				}
			}
			continue
		}
		rule := &client.StreamRule{
			Field:       rr.Field.ValueString(),
			Type:        int(rr.Type.ValueInt64()),
			Value:       rr.Value.ValueString(),
			Inverted:    rr.Inverted.ValueBool(),
			Description: rr.Description.ValueString(),
		}
		cr, err := r.client.WithContext(ctx).CreateStreamRule(streamID, rule)
		if err != nil {
			resp.Diagnostics.AddError("Error creating stream rule", err.Error())
			continue
		}
		if cr != nil && cr.ID != "" {
			plan.Rules[i].ID = types.StringValue(cr.ID)
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// validateStream performs basic checks for required fields and rule contents.
func validateStream(m *streamModel) (d diag.Diagnostics) {
	if m.Title.IsNull() || m.Title.IsUnknown() || m.Title.ValueString() == "" {
		d.AddAttributeError(path.Root("title"), "Invalid title", "Attribute 'title' must be a non-empty string.")
	}
	// Validate rules
	for i, r := range m.Rules {
		if r.Field.IsNull() || r.Field.IsUnknown() || r.Field.ValueString() == "" {
			d.AddAttributeError(path.Root("rule").AtListIndex(i).AtName("field"), "Invalid rule field", "Each rule must have non-empty 'field'.")
		}
		if r.Value.IsNull() || r.Value.IsUnknown() || r.Value.ValueString() == "" {
			d.AddAttributeError(path.Root("rule").AtListIndex(i).AtName("value"), "Invalid rule value", "Each rule must have non-empty 'value'.")
		}
		if r.Type.IsNull() || r.Type.IsUnknown() {
			d.AddAttributeError(path.Root("rule").AtListIndex(i).AtName("type"), "Invalid rule type", "Each rule must specify 'type' as a non-negative integer.")
		} else if r.Type.ValueInt64() < 0 {
			d.AddAttributeError(path.Root("rule").AtListIndex(i).AtName("type"), "Invalid rule type", "Rule 'type' must be >= 0.")
		}
	}
	return
}

func (r *streamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data streamModel
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

	if err := r.client.WithContext(ctx).DeleteStream(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting stream", err.Error())
	}
}

func (r *streamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	raw := req.ID
	if raw == "" {
		resp.Diagnostics.AddError("Empty import ID", "Provide a stream ID (UUID) or a title to import by title.")
		return
	}
	// If value looks like UUID or Mongo ObjectID (24 hex), pass through as id
	isUUID := regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString
	isHex24 := regexp.MustCompile(`(?i)^[0-9a-f]{24}$`).MatchString
	val := strings.TrimSpace(raw)
	if isUUID(val) || isHex24(val) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), val)...) //nolint:errcheck
		return
	}
	// Support explicit prefix title:My Stream Title
	const prefix = "title:"
	if strings.HasPrefix(strings.ToLower(val), prefix) {
		val = strings.TrimSpace(val[len(prefix):])
	}
	// Resolve by title via API
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Client is nil; cannot resolve stream by title during import.")
		return
	}
	list, err := r.client.WithContext(ctx).ListStreams()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list streams for import", err.Error())
		return
	}
	matches := make([]client.Stream, 0)
	for _, s := range list {
		if s.Title == val {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), matches[0].ID)...) //nolint:errcheck
		return
	}
	if len(matches) == 0 {
		resp.Diagnostics.AddError("Stream not found by title", "No stream found with exact title: "+val+". Provide a UUID or an exact title.")
		return
	}
	// Ambiguous
	resp.Diagnostics.AddError("Multiple streams match title", "Found "+fmt.Sprintf("%d", len(matches))+" streams with title '"+val+"'. Please import by UUID.")
}
