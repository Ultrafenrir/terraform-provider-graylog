package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"reflect"
	"testing"
)

func Test_toDashboardActionStrings(t *testing.T) {
	in := []types.String{types.StringValue("READ"), types.StringValue("edit"), types.StringValue("share"), types.StringValue("edit")}
	got := toDashboardActionStrings(in)
	want := []string{"edit", "read", "share"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	// default
	if d := toDashboardActionStrings(nil); !reflect.DeepEqual(d, []string{"read"}) {
		t.Fatalf("default actions want [read], got %v", d)
	}
}

func Test_toDashboardPermStrings(t *testing.T) {
	got := toDashboardPermStrings("abc123", []string{"read", "edit"})
	want := []string{"dashboards:read:abc123", "dashboards:edit:abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func Test_filterOutDashboardPerms(t *testing.T) {
	in := []string{"dashboards:read:abc", "users:read", "dashboards:edit:abc", "dashboards:share:def"}
	got := filterOutDashboardPerms(in, "abc")
	want := []string{"users:read", "dashboards:share:def"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func Test_detectActionsForDashboard(t *testing.T) {
	in := []string{"dashboards:read:abc", "dashboards:edit:abc", "users:read", "dashboards:share:def"}
	got := detectActionsForDashboard(in, "abc")
	want := []string{"edit", "read"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func Test_parseTwoPartDash(t *testing.T) {
	rn, id := parseTwoPartDash("Role/abc")
	if rn != "Role" || id != "abc" {
		t.Fatalf("unexpected: %s %s", rn, id)
	}
	rn, id = parseTwoPartDash("Role:abc")
	if rn != "Role" || id != "abc" {
		t.Fatalf("unexpected: %s %s", rn, id)
	}
}

func Test_dashboardSyntheticPermID(t *testing.T) {
	if dashboardSyntheticPermID("R", "D") != "R/D" {
		t.Fatalf("unexpected synthetic id")
	}
}
