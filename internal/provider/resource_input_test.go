package provider

import (
	"context"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestInputResource_New(t *testing.T) {
	r := NewInputResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}

func TestExtractorIdentityKey_StableAcrossCanonicalization(t *testing.T) {
	a := inputExtractorModel{
		Title:           types.StringValue("t1"),
		ExtractorType:   types.StringValue("regex"),
		SourceField:     types.StringValue("message"),
		TargetField:     types.StringValue("user"),
		CursorStrategy:  types.StringValue("copy"),
		ConditionType:   types.StringValue("none"),
		ExtractorConfig: types.StringValue(`{"b":2,"a":1}`),
	}
	b := inputExtractorModel{
		Title:           types.StringValue("t1"),
		ExtractorType:   types.StringValue("regex"),
		SourceField:     types.StringValue("message"),
		TargetField:     types.StringValue("user"),
		CursorStrategy:  types.StringValue("copy"),
		ConditionType:   types.StringValue("none"),
		ExtractorConfig: types.StringValue(`{"a":1,"b":2}`),
	}

	if diags := canonicalizeExtractorModel(&a); diags.HasError() {
		t.Fatalf("canonicalize a: %v", diags)
	}
	if diags := canonicalizeExtractorModel(&b); diags.HasError() {
		t.Fatalf("canonicalize b: %v", diags)
	}

	if extractorIdentityKey(a) != extractorIdentityKey(b) {
		t.Errorf("expected identical identity keys after canonicalization, got %q vs %q", extractorIdentityKey(a), extractorIdentityKey(b))
	}
}

func TestExtractorIdentityKey_DiffersWhenFieldChanges(t *testing.T) {
	base := inputExtractorModel{
		Title:         types.StringValue("t1"),
		ExtractorType: types.StringValue("regex"),
		SourceField:   types.StringValue("message"),
	}
	changed := base
	changed.SourceField = types.StringValue("full_message")

	if extractorIdentityKey(base) == extractorIdentityKey(changed) {
		t.Error("expected identity key to change when source_field changes")
	}
}

func TestExtractorModelClientRoundTrip(t *testing.T) {
	m := inputExtractorModel{
		Title:           types.StringValue("extract"),
		ExtractorType:   types.StringValue("regex"),
		SourceField:     types.StringValue("message"),
		TargetField:     types.StringValue("user"),
		CursorStrategy:  types.StringValue("copy"),
		ConditionType:   types.StringValue("none"),
		ExtractorConfig: types.StringValue(`{"regex_value":"user=(\\w+)"}`),
		Converters: []inputExtractorConverterModel{
			{Type: types.StringValue("lowercase")},
		},
	}

	ex, diags := extractorModelToClient(m)
	if diags.HasError() {
		t.Fatalf("extractorModelToClient: %v", diags)
	}
	if ex.Title != "extract" || ex.ExtractorType != "regex" || ex.SourceField != "message" || ex.TargetField != "user" {
		t.Fatalf("unexpected client.Extractor: %+v", ex)
	}
	if len(ex.Converters) != 1 || ex.Converters[0].Type != "lowercase" {
		t.Fatalf("expected one 'lowercase' converter, got %+v", ex.Converters)
	}
	ex.ID = "generated-id"

	back, diags := extractorClientToModel(*ex)
	if diags.HasError() {
		t.Fatalf("extractorClientToModel: %v", diags)
	}
	if back.ID.ValueString() != "generated-id" {
		t.Errorf("expected id 'generated-id', got %q", back.ID.ValueString())
	}
	if back.Title.ValueString() != "extract" {
		t.Errorf("expected title 'extract', got %q", back.Title.ValueString())
	}
	if len(back.Converters) != 1 || back.Converters[0].Type.ValueString() != "lowercase" {
		t.Errorf("expected one 'lowercase' converter, got %+v", back.Converters)
	}
}

func TestExtractorClientToModel_EmptyOptionalFieldsBecomeNull(t *testing.T) {
	ex := client.Extractor{
		ID:            "ex-1",
		Title:         "t",
		ExtractorType: "copy_input",
		SourceField:   "message",
		// TargetField, ConditionValue, ExtractorConfig intentionally left empty/nil.
	}
	m, diags := extractorClientToModel(ex)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.TargetField.IsNull() {
		t.Errorf("expected target_field to be null when empty, got %q", m.TargetField.ValueString())
	}
	if !m.ConditionValue.IsNull() {
		t.Errorf("expected condition_value to be null when empty, got %q", m.ConditionValue.ValueString())
	}
	if !m.ExtractorConfig.IsNull() {
		t.Errorf("expected extractor_config to be null when empty, got %q", m.ExtractorConfig.ValueString())
	}
	if m.CursorStrategy.ValueString() != "copy" {
		t.Errorf("expected cursor_strategy default 'copy', got %q", m.CursorStrategy.ValueString())
	}
	if m.ConditionType.ValueString() != "none" {
		t.Errorf("expected condition_type default 'none', got %q", m.ConditionType.ValueString())
	}
}

func TestExtractorModelToClient_DefaultsConvertersToEmptyList(t *testing.T) {
	m := inputExtractorModel{
		Title:          types.StringValue("extract"),
		ExtractorType:  types.StringValue("grok"),
		SourceField:    types.StringValue("message"),
		CursorStrategy: types.StringValue("copy"),
		ConditionType:  types.StringValue("none"),
	}

	ex, diags := extractorModelToClient(m)
	if diags.HasError() {
		t.Fatalf("extractorModelToClient: %v", diags)
	}
	if ex.Converters == nil {
		t.Fatal("expected converters to default to an empty, non-nil list")
	}
	if len(ex.Converters) != 0 {
		t.Fatalf("expected no converters, got %+v", ex.Converters)
	}
}

func TestResolveUnknownExtractorState(t *testing.T) {
	m := inputExtractorModel{
		ID:    types.StringUnknown(),
		Order: types.Int64Unknown(),
	}

	resolveUnknownExtractorState(&m)

	if !m.ID.IsNull() {
		t.Fatalf("expected an unknown id to become null, got %#v", m.ID)
	}
	if !m.Order.IsNull() {
		t.Fatalf("expected an unknown order to become null, got %#v", m.Order)
	}
}

func TestInputResource_UpgradeStateV6ToV7(t *testing.T) {
	ctx := context.Background()
	r := &inputResource{}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics)
	}

	upgraders := r.UpgradeState(ctx)
	upgrader, ok := upgraders[6]
	if !ok {
		t.Fatal("expected an upgrader registered for prior version 6")
	}
	if upgrader.PriorSchema == nil {
		t.Fatal("expected PriorSchema to be set for the v6 upgrader")
	}

	// Simulates what Read() used to persist: the full Graylog extractor object (including
	// server-only fields like creator_user_id/created_at) serialized as a JSON string.
	extractorsJSON := `[{"id":"ex-1","title":"extract user","extractor_type":"regex","source_field":"message","target_field":"user","cursor_strategy":"copy","extractor_config":{"regex_value":"user=(\\w+)"},"condition_type":"none","order":0,"creator_user_id":"admin","created_at":"2024-01-01T00:00:00.000Z"}]`

	priorType := upgrader.PriorSchema.Type().TerraformType(ctx)
	priorValue := tftypes.NewValue(priorType, map[string]tftypes.Value{
		"id":            tftypes.NewValue(tftypes.String, "input-123"),
		"title":         tftypes.NewValue(tftypes.String, "Test Input"),
		"type":          tftypes.NewValue(tftypes.String, "org.graylog2.inputs.gelf.udp.GELFUDPInput"),
		"global":        tftypes.NewValue(tftypes.Bool, true),
		"node":          tftypes.NewValue(tftypes.String, nil),
		"configuration": tftypes.NewValue(tftypes.String, `{"port":12201}`),
		"extractors":    tftypes.NewValue(tftypes.String, extractorsJSON),
		"timeouts": tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"create": tftypes.String,
				"update": tftypes.String,
				"delete": tftypes.String,
			},
		}, nil),
	})

	req := resource.UpgradeStateRequest{
		State: &tfsdk.State{
			Raw:    priorValue,
			Schema: *upgrader.PriorSchema,
		},
	}
	resp := resource.UpgradeStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}

	upgrader.StateUpgrader(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics)
	}

	var upgraded inputModel
	diags := resp.State.Get(ctx, &upgraded)
	if diags.HasError() {
		t.Fatalf("failed to get upgraded state: %v", diags)
	}

	if upgraded.ID.ValueString() != "input-123" {
		t.Errorf("expected id 'input-123', got %q", upgraded.ID.ValueString())
	}
	if len(upgraded.Extractors) != 1 {
		t.Fatalf("expected 1 extractor, got %d", len(upgraded.Extractors))
	}
	ex := upgraded.Extractors[0]
	if ex.ID.ValueString() != "ex-1" {
		t.Errorf("expected extractor id 'ex-1', got %q", ex.ID.ValueString())
	}
	if ex.Title.ValueString() != "extract user" {
		t.Errorf("expected title 'extract user', got %q", ex.Title.ValueString())
	}
	if ex.TargetField.ValueString() != "user" {
		t.Errorf("expected target_field 'user', got %q", ex.TargetField.ValueString())
	}
	if ex.ExtractorConfig.IsNull() || ex.ExtractorConfig.ValueString() == "" {
		t.Error("expected extractor_config to be populated")
	}
}
