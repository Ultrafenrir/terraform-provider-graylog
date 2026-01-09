package provider

import (
	"context"
	"errors"
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

	created, err := r.client.CreateStream(&client.Stream{
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
		cr, err := r.client.CreateStreamRule(data.ID.ValueString(), rule)
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
		if rules, err := r.client.ListStreamRules(data.ID.ValueString()); err == nil {
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
	s, err := r.client.GetStream(data.ID.ValueString())
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
	if rules, err := r.client.ListStreamRules(data.ID.ValueString()); err == nil {
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

	_, err := r.client.UpdateStream(data.ID.ValueString(), &client.Stream{
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Disabled:    data.Disabled.ValueBool(),
		IndexSetID:  data.IndexSetID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating stream", err.Error())
		return
	}
	// Resync rules: fetch existing, delete all, recreate from plan
	if existing, err := r.client.ListStreamRules(data.ID.ValueString()); err == nil {
		for _, ex := range existing {
			if ex.ID != "" {
				_ = r.client.DeleteStreamRule(data.ID.ValueString(), ex.ID)
			}
		}
	}
	for i, rr := range data.Rules {
		rule := &client.StreamRule{
			Field:       rr.Field.ValueString(),
			Type:        int(rr.Type.ValueInt64()),
			Value:       rr.Value.ValueString(),
			Inverted:    rr.Inverted.ValueBool(),
			Description: rr.Description.ValueString(),
		}
		cr, err := r.client.CreateStreamRule(data.ID.ValueString(), rule)
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

	if err := r.client.DeleteStream(data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting stream", err.Error())
	}
}

func (r *streamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
