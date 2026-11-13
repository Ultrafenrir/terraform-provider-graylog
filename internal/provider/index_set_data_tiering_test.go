package provider

import (
	"encoding/json"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const hotOnlyTiering = `{"type":"hot_only","index_lifetime_min":"P30D","index_lifetime_max":"P40D"}`

func TestValidateIndexSetTiering(t *testing.T) {
	for _, tc := range []struct {
		name      string
		data      types.String
		legacy    types.Bool
		rotation  *strategyModel
		wantError bool
	}{
		{"unmanaged", types.StringNull(), types.BoolNull(), nil, false},
		{"hot only", types.StringValue(hotOnlyTiering), types.BoolValue(false), nil, false},
		{"legacy", types.StringNull(), types.BoolValue(true), &strategyModel{}, false},
		{"unknown", types.StringUnknown(), types.BoolValue(false), nil, false},
		{"missing flag", types.StringValue(hotOnlyTiering), types.BoolNull(), nil, true},
		{"conflicting flag", types.StringValue(hotOnlyTiering), types.BoolValue(true), nil, true},
		{"missing document", types.StringNull(), types.BoolValue(false), nil, true},
		{"conflicting strategy", types.StringValue(hotOnlyTiering), types.BoolValue(false), &strategyModel{}, true},
		{"invalid json", types.StringValue("{"), types.BoolValue(false), nil, true},
		{"null", types.StringValue("null"), types.BoolValue(false), nil, true},
		{"array", types.StringValue("[]"), types.BoolValue(false), nil, true},
		{"missing bounds", types.StringValue(`{"type":"hot_only"}`), types.BoolValue(false), nil, true},
		{"unsupported type", types.StringValue(`{"type":"warm","index_lifetime_min":"P30D","index_lifetime_max":"P40D"}`), types.BoolValue(false), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := validateIndexSetTiering(&indexSetModel{DataTiering: tc.data, UseLegacyRotation: tc.legacy, Rotation: tc.rotation})
			if d.HasError() != tc.wantError {
				t.Fatalf("unexpected diagnostics: %v", d)
			}
		})
	}
}

func TestRefreshIndexSetTiering(t *testing.T) {
	pretty := "{\n  \"type\": \"hot_only\", \"index_lifetime_max\": \"P40D\", \"index_lifetime_min\": \"P30D\"\n}"
	for _, tc := range []struct {
		name       string
		prior      types.String
		legacy     types.Bool
		server     string
		importing  bool
		want       types.String
		wantLegacy types.Bool
	}{
		{"unmanaged remains null", types.StringNull(), types.BoolNull(), hotOnlyTiering, false, types.StringNull(), types.BoolNull()},
		{"formatting preserved", types.StringValue(pretty), types.BoolValue(false), hotOnlyTiering, false, types.StringValue(pretty), types.BoolValue(false)},
		{"server defaults ignored", types.StringValue(hotOnlyTiering), types.BoolValue(false), `{"type":"hot_only","index_lifetime_min":"P30D","index_lifetime_max":"P40D","extra":true}`, false, types.StringValue(hotOnlyTiering), types.BoolValue(false)},
		{"changed lifetime detected", types.StringValue(hotOnlyTiering), types.BoolValue(false), `{"type":"hot_only","index_lifetime_min":"P30D","index_lifetime_max":"P50D"}`, false, types.StringValue(`{"index_lifetime_max":"P50D","index_lifetime_min":"P30D","type":"hot_only"}`), types.BoolValue(false)},
		{"tiering removed externally", types.StringValue(hotOnlyTiering), types.BoolValue(false), "null", false, types.StringNull(), types.BoolValue(true)},
		{"import", types.StringNull(), types.BoolNull(), hotOnlyTiering, true, types.StringValue(`{"index_lifetime_max":"P40D","index_lifetime_min":"P30D","type":"hot_only"}`), types.BoolValue(false)},
		{"old server import", types.StringNull(), types.BoolNull(), "", true, types.StringNull(), types.BoolNull()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := indexSetModel{DataTiering: tc.prior, UseLegacyRotation: tc.legacy}
			d := refreshIndexSetTiering(&data, &client.IndexSet{DataTiering: json.RawMessage(tc.server)}, tc.importing)
			if d.HasError() {
				t.Fatal(d)
			}
			if !data.DataTiering.Equal(tc.want) || !data.UseLegacyRotation.Equal(tc.wantLegacy) {
				t.Fatalf("got data=%s legacy=%s; want data=%s legacy=%s", data.DataTiering, data.UseLegacyRotation, tc.want, tc.wantLegacy)
			}
		})
	}
}

func TestRefreshIndexSetTiering_LegacyImportPreservesExistingStateShape(t *testing.T) {
	legacy := true
	data := indexSetModel{DataTiering: types.StringNull(), UseLegacyRotation: types.BoolNull()}
	d := refreshIndexSetTiering(&data, &client.IndexSet{UseLegacyRotation: &legacy}, true)
	if d.HasError() || !data.UseLegacyRotation.IsNull() || !data.DataTiering.IsNull() {
		t.Fatalf("legacy import must not add tiering attributes: %v, %+v", d, data)
	}
}
