package provider

import (
	"encoding/json"
	"testing"
)

func TestCanonicalizeJSONFromString_SortsKeys(t *testing.T) {
	in1 := `{"b":2,"a":1}`
	in2 := `{"a":1,"b":2}`

	c1, err := CanonicalizeJSONFromString(in1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := CanonicalizeJSONFromString(in2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("canonical outputs differ: %s vs %s", c1, c2)
	}
	if c1 != `{"a":1,"b":2}` {
		t.Fatalf("unexpected canonical: %s", c1)
	}
}

func TestCanonicalizeJSONValue_Nested(t *testing.T) {
	var v interface{}
	if err := json.Unmarshal([]byte(`{"x":[{"b":2,"a":1},{"k":"v"}]}`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := CanonicalizeJSONValue(v)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	want := `{"x":[{"a":1,"b":2},{"k":"v"}]}`
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestCanonicalizeJSONFromString_ArrayStable(t *testing.T) {
	in := `[3,2,1]`
	out, err := CanonicalizeJSONFromString(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in { // arrays preserve order
		t.Fatalf("want %s, got %s", in, out)
	}
}
