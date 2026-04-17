//go:build integration

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// minimal harness to call Read() and verify normalization
func TestIndexSet_Read_NormalizesDefaultsAndLegacyNull(t *testing.T) {
	// Сервер вернёт IndexSet с нулевыми/пустыми значениями — провайдер должен подставить дефолты
	is := client.IndexSet{
		ID:            "id1",
		Title:         "T",
		IndexPrefix:   "p",
		Shards:        1,
		Replicas:      0,
		IndexAnalyzer: "", // должен стать "standard"
		// FieldTypeRefreshInterval: 0 -> 5000
		// IndexOptimizationMaxNumSegments: 0 -> 1
		IndexOptimizationDisabled: false,
		RotationStrategyClass:     "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy",
		RotationStrategyConfig:    map[string]any{"type": "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig", "max_docs_per_index": 10},
		RetentionStrategyClass:    "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy",
		RetentionStrategyConfig:   map[string]any{"type": "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig", "max_number_of_indices": 2},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/indices/index_sets/id1":
			_ = json.NewEncoder(w).Encode(is)
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	c := &client.Client{BaseURL: ts.URL, HTTP: &http.Client{Timeout: 2 * time.Second}, MaxRetries: 0}
	r := &indexSetResource{client: c}

	// Сформируем минимальное состояние с id
	state := indexSetModel{ID: types.StringValue("id1")}
	ctx := context.Background()

	// Обёртки запрос/ответ фреймворка
	req := fwresource.ReadRequest{}
	var resp fwresource.ReadResponse

	r.Read(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %+v", resp.Diagnostics)
	}
	// Распакуем новое состояние и проверим нормализацию
	var out indexSetModel
	if diags := req.State.Get(ctx, &out); diags.HasError() {
		t.Fatalf("state get: %+v", diags)
	}

	if out.IndexAnalyzer.ValueString() != "standard" {
		t.Fatalf("analyzer want 'standard', got %q", out.IndexAnalyzer.ValueString())
	}
	if v := out.FieldTypeRefresh.ValueInt64(); v != 5000 {
		t.Fatalf("field_type_refresh_interval want 5000, got %d", v)
	}
	if v := out.IndexOptMaxSeg.ValueInt64(); v != 1 {
		t.Fatalf("index_optimization_max_num_segments want 1, got %d", v)
	}
	if !out.RotationStrategy.IsNull() || !out.RetentionStrategy.IsNull() {
		t.Fatalf("legacy strategies must be null to avoid drift")
	}
	if out.Rotation == nil || out.Retention == nil {
		t.Fatalf("rotation/retention blocks must be set")
	}
}

// fake state implements tfsdk.StateCompatible to back the Read call without full framework harness
type fakeState[T any] struct{ v T }

func newFakeState[T any](v T) *fakeState[T] { return &fakeState[T]{v: v} }

func (s *fakeState[T]) Get(ctx context.Context, target any) (diags diagWrap) {
	b, _ := json.Marshal(s.v)
	_ = json.Unmarshal(b, target)
	return
}
func (s *fakeState[T]) Set(ctx context.Context, src any) (diags diagWrap) {
	b, _ := json.Marshal(src)
	var v T
	_ = json.Unmarshal(b, &v)
	s.v = v
	return
}

// minimal diag wrapper to satisfy interface in tests
type diagWrap struct{}

func (d diagWrap) HasError() bool { return false }
