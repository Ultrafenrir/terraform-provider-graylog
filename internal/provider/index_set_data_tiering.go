package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithValidateConfig = (*indexSetResource)(nil)

func (r *indexSetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data indexSetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(validateIndexSetTiering(&data)...)
	}
}

func validateIndexSetTiering(data *indexSetModel) (diags diag.Diagnostics) {
	if data.DataTiering.IsUnknown() || data.UseLegacyRotation.IsUnknown() {
		return
	}
	if data.DataTiering.IsNull() {
		if !data.UseLegacyRotation.IsNull() && !data.UseLegacyRotation.ValueBool() {
			diags.AddAttributeError(path.Root("data_tiering"), "Missing tiering configuration", "Set data_tiering when use_legacy_rotation is false.")
		}
		return
	}
	if data.UseLegacyRotation.IsNull() || data.UseLegacyRotation.ValueBool() {
		diags.AddAttributeError(path.Root("use_legacy_rotation"), "Conflicting rotation mode", "Set use_legacy_rotation = false with data_tiering. To return to legacy rotation, remove data_tiering and set use_legacy_rotation = true.")
	}
	if data.Rotation != nil || data.Retention != nil {
		diags.AddAttributeError(path.Root("data_tiering"), "Conflicting strategies", "Data tiering manages rotation and retention. Remove the rotation and retention blocks when setting data_tiering.")
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(data.DataTiering.ValueString()), &document); err != nil || document == nil {
		diags.AddAttributeError(path.Root("data_tiering"), "Invalid tiering configuration", "data_tiering must be a JSON object with type, index_lifetime_min and index_lifetime_max.")
		return
	}
	if document["type"] != "hot_only" {
		diags.AddAttributeError(path.Root("data_tiering"), "Unsupported tiering type", "Only type = hot_only is currently supported for configuration. Existing server tiering is preserved when both tiering attributes are omitted.")
	}
	for _, key := range []string{"index_lifetime_min", "index_lifetime_max"} {
		value, ok := document[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			diags.AddAttributeError(path.Root("data_tiering"), "Missing index lifetime", key+" must be a non-empty ISO-8601 period string, for example P30D. Graylog validates the period and lifetime bounds.")
		}
	}
	return
}

func indexSetTieringJSON(value types.String) json.RawMessage {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return json.RawMessage(value.ValueString())
}

func indexSetLegacyRotation(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueBool()
	return &result
}

// Keep unconfigured attributes null, so existing configurations do not acquire
// tiering state or start managing a server default after upgrading the provider.
// Preserve the exact configured JSON when only formatting or server-added keys
// differ, but expose changes to managed keys as drift.
func refreshIndexSetTiering(data *indexSetModel, server *client.IndexSet, importing bool) (diags diag.Diagnostics) {
	if (importing && server.HasDataTiering()) || !data.UseLegacyRotation.IsNull() {
		if server.UseLegacyRotation != nil {
			data.UseLegacyRotation = types.BoolValue(*server.UseLegacyRotation)
		} else if server.HasDataTiering() || !data.UseLegacyRotation.IsNull() {
			data.UseLegacyRotation = types.BoolValue(!server.HasDataTiering())
		}
	}
	if !importing && data.DataTiering.IsNull() {
		return
	}
	if !server.HasDataTiering() {
		data.DataTiering = types.StringNull()
		return
	}
	mask := data.DataTiering.ValueString()
	projected, err := ProjectAndCanonicalizeJSON(string(server.DataTiering), mask)
	if err != nil {
		diags.AddError("Invalid data tiering response", err.Error())
		return
	}
	previous, err := CanonicalizeJSONFromString(mask)
	if err != nil || previous != projected {
		data.DataTiering = types.StringValue(projected)
	}
	return
}
