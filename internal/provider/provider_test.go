package provider

import (
	"testing"
)

func TestProvider_New(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("provider is nil")
	}
}
