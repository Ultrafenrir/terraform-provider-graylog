package provider

import "testing"

func TestDashboardResource_New(t *testing.T) {
	r := NewDashboardResource()
	if r == nil {
		t.Fatal("expected non-nil resource")
	}
}
