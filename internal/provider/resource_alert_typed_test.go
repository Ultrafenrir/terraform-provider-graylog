package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func Test_buildEventConfigFromModel_TypedThreshold(t *testing.T) {
	m := alertModelV2{Threshold: &alertThresholdBlockModel{}}
	// Fill typed threshold
	m.Threshold.Query = typesString("level:ERROR")
	m.Threshold.SearchWithinMs = typesInt64(5 * 60 * 1000)
	m.Threshold.ExecuteEveryMs = typesInt64(1 * 60 * 1000)
	m.Threshold.GraceMs = typesInt64(30 * 1000)
	m.Threshold.BacklogSize = typesInt64(10)
	m.Threshold.GroupBy = []types.String{types.StringValue("source")}
	m.Threshold.Series = []alertSeriesModel{{
		ID:       typesString("count"),
		Function: typesString("count()"),
	}}
	m.Threshold.Threshold.Type = typesString("more")
	m.Threshold.Threshold.Value = typesFloat64(100)
	m.Threshold.Execution.Interval.Type = typesString("interval")
	m.Threshold.Execution.Interval.Value = typesInt64(1)
	m.Threshold.Execution.Interval.Unit = typesString("MINUTES")
	m.Threshold.Filter.Streams = []types.String{typesString("s-1"), typesString("s-2")}

	cfg, diags := buildEventConfigFromModel(context.Background(), &m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if got := cfg["type"]; got != "threshold-v1" {
		t.Fatalf("type mismatch: want threshold-v1, got %v", got)
	}
	if got := cfg["query"]; got != "level:ERROR" {
		t.Fatalf("query mismatch: %v", got)
	}
	if got := cfg["search_within_ms"]; got != int64(5*60*1000) {
		t.Fatalf("search_within_ms mismatch: %v", got)
	}
	if got := cfg["execute_every_ms"]; got != int64(1*60*1000) {
		t.Fatalf("execute_every_ms mismatch: %v", got)
	}
	if got := cfg["grace_ms"]; !intEqual(got, 30*1000) {
		t.Fatalf("grace_ms mismatch: %v", got)
	}
	if got := cfg["backlog_size"]; !intEqual(got, 10) {
		t.Fatalf("backlog_size mismatch: %v", got)
	}
	if th, ok := cfg["threshold"].(map[string]interface{}); ok {
		if th["type"] != "more" || !floatEqual(th["value"], 100.0) {
			t.Fatalf("threshold mismatch: %+v", th)
		}
	} else {
		t.Fatalf("threshold not present")
	}
	if streams, ok := cfg["streams"].([]string); ok {
		if len(streams) != 2 || streams[0] != "s-1" || streams[1] != "s-2" {
			t.Fatalf("streams mismatch: %+v", streams)
		}
	} else if raw, ok := cfg["streams"].([]interface{}); ok {
		if len(raw) != 2 || raw[0] != "s-1" || raw[1] != "s-2" {
			t.Fatalf("streams mismatch (iface): %+v", raw)
		}
	} else {
		t.Fatalf("streams not present or wrong type: %T", cfg["streams"])
	}
	if s, ok := cfg["series"].([]map[string]interface{}); ok {
		if len(s) != 1 || s[0]["function"] != "count()" {
			t.Fatalf("series mismatch: %+v", s)
		}
	} else if s2, ok := cfg["series"].([]interface{}); ok {
		// depending on map type, the slice may be []interface{}
		if len(s2) != 1 {
			t.Fatalf("series size mismatch: %+v", s2)
		}
	} else {
		t.Fatalf("series not present or wrong type: %T", cfg["series"])
	}
	if ex, ok := cfg["execution"].(map[string]interface{}); ok {
		if in, ok := ex["interval"].(map[string]interface{}); ok {
			if in["type"] != "interval" || !intEqual(in["value"], 1) || in["unit"] != "MINUTES" {
				t.Fatalf("execution.interval mismatch: %+v", in)
			}
		} else {
			t.Fatalf("execution.interval missing: %+v", ex)
		}
	} else {
		t.Fatalf("execution missing")
	}
}

func Test_buildEventConfigFromModel_FallbackConfig(t *testing.T) {
	m := alertModelV2{}
	m.Config = typesString("{\"type\":\"aggregation-v1\",\"query\":\"*\"}")
	cfg, diags := buildEventConfigFromModel(context.Background(), &m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if !reflect.DeepEqual(cfg["type"], "aggregation-v1") {
		t.Fatalf("expected fallback to config JSON, got: %v", cfg)
	}
}

func Test_buildEventConfigFromModel_TypedAggregation(t *testing.T) {
	m := alertModelV2{Aggregation: &alertAggregationBlockModel{}}
	// Fill typed aggregation
	m.Aggregation.Query = typesString("level:ERROR")
	m.Aggregation.SearchWithinMs = typesInt64(10 * 60 * 1000)
	m.Aggregation.ExecuteEveryMs = typesInt64(2 * 60 * 1000)
	m.Aggregation.GraceMs = typesInt64(15 * 1000)
	m.Aggregation.BacklogSize = typesInt64(3)
	m.Aggregation.GroupBy = []types.String{types.StringValue("source")}
	m.Aggregation.Series = []alertSeriesModel{{
		ID:       typesString("count"),
		Function: typesString("count()"),
	}}
	// optional threshold
	m.Aggregation.Threshold.Type = typesString("more")
	m.Aggregation.Threshold.Value = typesFloat64(5)
	m.Aggregation.Execution.Interval.Type = typesString("interval")
	m.Aggregation.Execution.Interval.Value = typesInt64(1)
	m.Aggregation.Execution.Interval.Unit = typesString("MINUTES")
	m.Aggregation.Filter.Streams = []types.String{typesString("s-a"), typesString("s-b")}

	cfg, diags := buildEventConfigFromModel(context.Background(), &m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if got := cfg["type"]; got != "aggregation-v1" {
		t.Fatalf("type mismatch: want aggregation-v1, got %v", got)
	}
	if got := cfg["query"]; got != "level:ERROR" {
		t.Fatalf("query mismatch: %v", got)
	}
	if got := cfg["search_within_ms"]; !intEqual(got, 10*60*1000) {
		t.Fatalf("search_within_ms mismatch: %v", got)
	}
	if got := cfg["execute_every_ms"]; !intEqual(got, 2*60*1000) {
		t.Fatalf("execute_every_ms mismatch: %v", got)
	}
	if got := cfg["grace_ms"]; !intEqual(got, 15*1000) {
		t.Fatalf("grace_ms mismatch: %v", got)
	}
	if got := cfg["backlog_size"]; !intEqual(got, 3) {
		t.Fatalf("backlog_size mismatch: %v", got)
	}
	if th, ok := cfg["threshold"].(map[string]interface{}); ok {
		if th["type"] != "more" || !floatEqual(th["value"], 5.0) {
			t.Fatalf("threshold mismatch: %+v", th)
		}
	} else {
		t.Fatalf("threshold not present for aggregation")
	}
	if streams, ok := cfg["streams"].([]string); ok {
		if len(streams) != 2 || streams[0] != "s-a" || streams[1] != "s-b" {
			t.Fatalf("streams mismatch: %+v", streams)
		}
	} else if raw, ok := cfg["streams"].([]interface{}); ok {
		if len(raw) != 2 || raw[0] != "s-a" || raw[1] != "s-b" {
			t.Fatalf("streams mismatch (iface): %+v", raw)
		}
	} else {
		t.Fatalf("streams not present or wrong type: %T", cfg["streams"])
	}
	if ex, ok := cfg["execution"].(map[string]interface{}); ok {
		if in, ok := ex["interval"].(map[string]interface{}); ok {
			if in["type"] != "interval" || !intEqual(in["value"], 1) || in["unit"] != "MINUTES" {
				t.Fatalf("execution.interval mismatch: %+v", in)
			}
		} else {
			t.Fatalf("execution.interval missing: %+v", ex)
		}
	} else {
		t.Fatalf("execution missing")
	}
}

func Test_buildEventConfigFromModel_Validation_Negatives(t *testing.T) {
	// Threshold: negative grace
	mt := alertModelV2{Threshold: &alertThresholdBlockModel{}}
	mt.Threshold.Threshold.Type = typesString("more")
	mt.Threshold.Threshold.Value = typesFloat64(1)
	mt.Threshold.GraceMs = typesInt64(-1)
	if _, diags := buildEventConfigFromModel(context.Background(), &mt); !diags.HasError() {
		t.Fatalf("expected diagnostics error for negative grace_ms in threshold")
	}

	// Aggregation: negative backlog
	ma := alertModelV2{Aggregation: &alertAggregationBlockModel{}}
	ma.Aggregation.Query = typesString("*")
	ma.Aggregation.BacklogSize = typesInt64(-5)
	if _, diags := buildEventConfigFromModel(context.Background(), &ma); !diags.HasError() {
		t.Fatalf("expected diagnostics error for negative backlog_size in aggregation")
	}
}

// helpers for typed values in tests
func typesString(s string) types.String    { return types.StringValue(s) }
func typesInt64(v int64) types.Int64       { return types.Int64Value(v) }
func typesFloat64(f float64) types.Float64 { return types.Float64Value(f) }

func floatEqual(v interface{}, want float64) bool {
	switch x := v.(type) {
	case float64:
		return x == want
	case int64:
		return float64(x) == want
	case int:
		return float64(x) == want
	default:
		return false
	}
}

func intEqual(v interface{}, want int64) bool {
	switch x := v.(type) {
	case float64:
		return int64(x) == want
	case int64:
		return x == want
	case int:
		return int64(x) == want
	default:
		return false
	}
}
