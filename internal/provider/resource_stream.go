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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type streamResource struct{ client *client.Client }

type streamRuleModel struct {
	ID          types.String `tfsdk:"id"`
	Field       types.String `tfsdk:"field"`
	Type        types.Int64  `tfsdk:"type"`
	Value       types.String `tfsdk:"value"`
	Inverted    types.Bool   `tfsdk:"inverted"`
	Description types.String `tfsdk:"description"`
}

type streamModel struct {
	ID          types.String      `tfsdk:"id"`
	Title       types.String      `tfsdk:"title"`
	Description types.String      `tfsdk:"description"`
	Disabled    types.Bool        `tfsdk:"disabled"`
	IndexSetID  types.String      `tfsdk:"index_set_id"`
	Rules       []streamRuleModel `tfsdk:"rule"`
	Timeouts    timeouts.Value    `tfsdk:"timeouts"`
}

func NewStreamResource() resource.Resource { return &streamResource{} }

func (r *streamResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_stream"
}

func (r *streamResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     2,
		Description: "Manages a Graylog stream resource. Compatible with Graylog v5, v6, and v7.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, Description: "The unique identifier of the stream"},
			"title":        schema.StringAttribute{Required: true, Description: "The title of the stream"},
			"description":  schema.StringAttribute{Optional: true, Description: "Description of the stream"},
			"disabled":     schema.BoolAttribute{Optional: true, Description: "Whether the stream is disabled"},
			"index_set_id": schema.StringAttribute{Optional: true, Description: "The index set ID to use for this stream"},
			"timeouts":     timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
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

	created, err := r.client.WithContext(ctx).CreateStream(&client.Stream{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Disabled:    data.Disabled.ValueBool(),
		IndexSetID:  data.IndexSetID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating stream", err.Error())
		return
	}
	data.ID = types.StringValue(created.ID)
	// Create rules if provided via dedicated API
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
			return
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
	data.Description = types.StringValue(s.Description)
	data.Disabled = types.BoolValue(s.Disabled)
	data.IndexSetID = types.StringValue(s.IndexSetID)
	// Read stream rules via API
	if rules, err := r.client.WithContext(ctx).ListStreamRules(data.ID.ValueString()); err == nil {
		out := make([]streamRuleModel, 0, len(rules))
		for _, rrule := range rules {
			out = append(out, streamRuleModel{
				ID:          types.StringValue(rrule.ID),
				Field:       types.StringValue(rrule.Field),
				Type:        types.Int64Value(int64(rrule.Type)),
				Value:       types.StringValue(rrule.Value),
				Inverted:    types.BoolValue(rrule.Inverted),
				Description: types.StringValue(rrule.Description),
			})
		}
		data.Rules = out
	} else {
		resp.Diagnostics.AddWarning("Unable to read stream rules", err.Error())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *streamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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
	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	_, err := r.client.WithContext(ctx).UpdateStream(data.ID.ValueString(), &client.Stream{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Disabled:    data.Disabled.ValueBool(),
		IndexSetID:  data.IndexSetID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating stream", err.Error())
		return
	}
	// Diff-aware sync of rules: delete extra, create missing; keep matching ones
	// Build maps by stable key
	existing, err := r.client.WithContext(ctx).ListStreamRules(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing stream rules", err.Error())
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
	for _, rr := range data.Rules {
		k := makeKey(rr.Field.ValueString(), int(rr.Type.ValueInt64()), rr.Value.ValueString(), rr.Inverted.ValueBool(), rr.Description.ValueString())
		desiredKeys[k] = struct{}{}
	}
	// Delete rules that are not desired
	for _, ex := range existing {
		k := makeKey(ex.Field, ex.Type, ex.Value, ex.Inverted, ex.Description)
		if _, ok := desiredKeys[k]; !ok {
			if ex.ID != "" {
				_ = r.client.WithContext(ctx).DeleteStreamRule(data.ID.ValueString(), ex.ID)
			}
		}
	}
	// Create rules that are missing
	for i, rr := range data.Rules {
		k := makeKey(rr.Field.ValueString(), int(rr.Type.ValueInt64()), rr.Value.ValueString(), rr.Inverted.ValueBool(), rr.Description.ValueString())
		if _, ok := exByKey[k]; ok {
			// Already present — if ID known in state, keep it; otherwise fill from map
			if data.Rules[i].ID.IsNull() || data.Rules[i].ID.IsUnknown() || data.Rules[i].ID.ValueString() == "" {
				if id := exByKey[k]; id != "" {
					data.Rules[i].ID = types.StringValue(id)
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
		cr, err := r.client.WithContext(ctx).CreateStreamRule(data.ID.ValueString(), rule)
		if err != nil {
			resp.Diagnostics.AddError("Error creating stream rule", err.Error())
			return
		}
		if cr != nil && cr.ID != "" {
			data.Rules[i].ID = types.StringValue(cr.ID)
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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
