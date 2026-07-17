package provider

import (
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Проверяем нормализацию дефолтов и что "type" поле фильтруется из rotation/retention config
func TestIndexSet_NormalizesDefaultsAndFiltersTypeField(t *testing.T) {
	is := client.IndexSet{
		ID:                        "id1",
		Title:                     "T",
		IndexPrefix:               "p",
		Shards:                    1,
		Replicas:                  0,
		IndexAnalyzer:             "", // должен стать "standard"
		IndexOptimizationDisabled: false,
		RotationStrategyClass:     "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy",
		RotationStrategyConfig:    map[string]any{"type": "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig", "max_docs_per_index": 10},
		RetentionStrategyClass:    "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy",
		RetentionStrategyConfig:   map[string]any{"type": "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig", "max_number_of_indices": 2},
	}

	data := indexSetModel{ID: types.StringValue("id1")}
	// Используем helper, который применяет логику Read
	applyIndexSetReadState(ctxBackground(), &data, &is)

	if data.IndexAnalyzer.ValueString() != "standard" {
		t.Fatalf("analyzer want 'standard', got %q", data.IndexAnalyzer.ValueString())
	}
	if v := data.FieldTypeRefresh.ValueInt64(); v != 5000 {
		t.Fatalf("field_type_refresh_interval want 5000, got %d", v)
	}
	if v := data.IndexOptMaxSeg.ValueInt64(); v != 1 {
		t.Fatalf("index_optimization_max_num_segments want 1, got %d", v)
	}
	if data.Rotation == nil || data.Retention == nil {
		t.Fatalf("rotation/retention blocks must be set")
	}
	// Verify that "type" field is filtered out from config
	rotationConfig := make(map[string]string)
	data.Rotation.Config.ElementsAs(ctxBackground(), &rotationConfig, false)
	if _, hasType := rotationConfig["type"]; hasType {
		t.Fatalf("rotation config should not contain 'type' field")
	}
	if rotationConfig["max_docs_per_index"] != "10" {
		t.Fatalf("rotation config max_docs_per_index want '10', got %q", rotationConfig["max_docs_per_index"])
	}
}

// Guards the fix for "Provider produced inconsistent result after apply: .description: was
// null, but now cty.StringVal(\"\")" — an unconfigured (null) description must stay null when
// the server reports an empty description, while an explicitly-cleared ("") description must
// stay "" (both are valid Terraform-side representations of "no description", and the server
// can't tell them apart, so applyIndexSetReadState must consult the caller's prior value).
func TestIndexSet_DescriptionNullVsEmptyPreserved(t *testing.T) {
	is := &client.IndexSet{ID: "id1", Title: "T", IndexPrefix: "p", Description: ""}

	t.Run("unconfigured stays null", func(t *testing.T) {
		data := indexSetModel{ID: types.StringValue("id1"), Description: types.StringNull()}
		applyIndexSetReadState(ctxBackground(), &data, is)
		if !data.Description.IsNull() {
			t.Fatalf("expected description to stay null, got %q", data.Description.ValueString())
		}
	})

	t.Run("explicitly cleared stays empty string", func(t *testing.T) {
		data := indexSetModel{ID: types.StringValue("id1"), Description: types.StringValue("")}
		applyIndexSetReadState(ctxBackground(), &data, is)
		if data.Description.IsNull() {
			t.Fatal("expected description to stay \"\", got null")
		}
		if data.Description.ValueString() != "" {
			t.Fatalf("expected description \"\", got %q", data.Description.ValueString())
		}
	})

	t.Run("non-empty server value always wins", func(t *testing.T) {
		withDesc := &client.IndexSet{ID: "id1", Title: "T", IndexPrefix: "p", Description: "hello"}
		data := indexSetModel{ID: types.StringValue("id1"), Description: types.StringNull()}
		applyIndexSetReadState(ctxBackground(), &data, withDesc)
		if data.Description.ValueString() != "hello" {
			t.Fatalf("expected description 'hello', got %q", data.Description.ValueString())
		}
	})
}

// маленький помощник, чтобы не тащить context в тест
func ctxBackground() contextLike { return contextLike{} }

// минимальный контекст для вызова map/value конструкторов (совместим по интерфейсу)
type contextLike struct{}

func (contextLike) Deadline() (deadline time.Time, ok bool) { return }
func (contextLike) Done() <-chan struct{}                   { return nil }
func (contextLike) Err() error                              { return nil }
func (contextLike) Value(key any) any                       { return nil }
