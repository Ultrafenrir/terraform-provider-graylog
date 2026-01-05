package provider

import "testing"

func TestAlertResource_New(t *testing.T) {
	r := NewAlertResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}
