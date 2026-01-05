package provider

import (
	"testing"
)

func TestInputResource_New(t *testing.T) {
	r := NewInputResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}
