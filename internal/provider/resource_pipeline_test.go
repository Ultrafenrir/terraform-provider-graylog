package provider

import "testing"

func TestPipelineResource_New(t *testing.T) {
	r := NewPipelineResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}
