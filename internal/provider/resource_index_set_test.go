package provider

import (
	"testing"
)

func TestIndexSetResource_New(t *testing.T) {
	r := NewIndexSetResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}
