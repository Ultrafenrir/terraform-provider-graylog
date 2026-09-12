package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The password is pushed to Graylog only when the plan differs from state:
// an unchanged, removed or never-set password must not reach the /password
// endpoint, which answers 403 for external (LDAP/AD) users.
func TestPasswordToSet(t *testing.T) {
	for _, tc := range []struct {
		name        string
		plan, state types.String
		want        string
	}{
		{"unchanged", types.StringValue("x"), types.StringValue("x"), ""},
		{"changed", types.StringValue("y"), types.StringValue("x"), "y"},
		{"removed from config", types.StringNull(), types.StringValue("x"), ""},
		{"never set (imported)", types.StringNull(), types.StringNull(), ""},
		{"unknown", types.StringUnknown(), types.StringValue("x"), ""},
		{"set after import", types.StringValue("x"), types.StringNull(), "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := passwordToSet(tc.plan, tc.state); got != tc.want {
				t.Fatalf("passwordToSet(%v, %v) = %q, want %q", tc.plan, tc.state, got, tc.want)
			}
		})
	}
}

// Removing the password from the configuration keeps the state value (so
// re-adding the same one is a no-op), and a null state stays null instead
// of becoming "(known after apply)".
func TestKeepStateWhenUnset(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		config, state, plan, want types.String
	}{
		{"unset keeps state", types.StringNull(), types.StringValue("x"), types.StringUnknown(), types.StringValue("x")},
		{"unset with null state stays null", types.StringNull(), types.StringNull(), types.StringUnknown(), types.StringNull()},
		{"configured value wins", types.StringValue("y"), types.StringValue("x"), types.StringValue("y"), types.StringValue("y")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			keepStateWhenUnset{}.PlanModifyString(context.Background(), planmodifier.StringRequest{ConfigValue: tc.config, StateValue: tc.state, PlanValue: tc.plan}, resp)
			if !resp.PlanValue.Equal(tc.want) {
				t.Fatalf("plan = %v, want %v", resp.PlanValue, tc.want)
			}
		})
	}
}
