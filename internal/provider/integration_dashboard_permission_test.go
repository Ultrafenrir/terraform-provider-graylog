//go:build integration

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

// TestIntegration_DashboardPermissionRoleBinding verifies that role permissions
// with the pattern dashboards:<action>:<dashboard_id> are accepted by Graylog.
// Skips on Graylog 5.x and 6.x where classic dashboards CRUD is unavailable in many images.
func TestIntegration_DashboardPermissionRoleBinding(t *testing.T) {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("integration env is not configured: set URL and TOKEN env vars")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := ic.New(baseURL, token)

	// Guard: many GL 5.x/6.x images do not support classic /dashboards CRUD
	if c.APIVersion == ic.APIV5 || c.APIVersion == ic.APIV6 {
		t.Skip("dashboard CRUD/permissions are not supported in this Graylog version/image; skipping test")
	}

	// Unique suffix
	suf := time.Now().UnixNano()

	// Create a role
	roleName := fmt.Sprintf("tf-itest-role-dashperm-%d", suf)
	role, err := c.CreateRole(&ic.Role{
		Name:        roleName,
		Description: "integration role for dashboard permissions",
		Permissions: []string{},
	})
	if err != nil {
		t.Fatalf("CreateRole error: %v", err)
	}
	defer func() { _ = c.DeleteRole(role.Name) }()

	// Create a dashboard
	dash, err := c.CreateDashboard(&ic.Dashboard{
		Title:       fmt.Sprintf("tf-itest-dashperm-%d", suf),
		Description: "integration dashboard for permissions",
	})
	if err != nil {
		t.Fatalf("CreateDashboard error: %v", err)
	}
	if dash.ID == "" {
		t.Fatalf("expected dashboard ID to be set")
	}
	defer func() { _ = c.DeleteDashboard(dash.ID) }()

	// 1) Grant read
	if _, err := c.UpdateRole(role.Name, &ic.Role{
		Description: role.Description,
		Permissions: []string{fmt.Sprintf("dashboards:read:%s", dash.ID)},
		ReadOnly:    role.ReadOnly,
	}); err != nil {
		t.Fatalf("UpdateRole (grant read) error: %v", err)
	}
	r1, err := c.GetRole(role.Name)
	if err != nil {
		t.Fatalf("GetRole after grant read error: %v", err)
	}
	found := false
	for _, p := range r1.Permissions {
		if p == fmt.Sprintf("dashboards:read:%s", dash.ID) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dashboards:read:%s permission present, got: %+v", dash.ID, r1.Permissions)
	}

	// 2) Upgrade to read+edit
	if _, err := c.UpdateRole(role.Name, &ic.Role{
		Description: role.Description,
		Permissions: []string{
			fmt.Sprintf("dashboards:read:%s", dash.ID),
			fmt.Sprintf("dashboards:edit:%s", dash.ID),
		},
		ReadOnly: role.ReadOnly,
	}); err != nil {
		t.Fatalf("UpdateRole (grant read+edit) error: %v", err)
	}
	r2, err := c.GetRole(role.Name)
	if err != nil {
		t.Fatalf("GetRole after grant read+edit error: %v", err)
	}
	need := map[string]bool{
		fmt.Sprintf("dashboards:read:%s", dash.ID): true,
		fmt.Sprintf("dashboards:edit:%s", dash.ID): true,
	}
	for _, p := range r2.Permissions {
		delete(need, p)
	}
	if len(need) != 0 {
		t.Fatalf("expected both read and edit perms; missing: %+v; got: %+v", need, r2.Permissions)
	}

	// 3) Remove all dashboard-scoped perms (cleanup path)
	filtered := make([]string, 0, len(r2.Permissions))
	for _, p := range r2.Permissions {
		if p == fmt.Sprintf("dashboards:read:%s", dash.ID) || p == fmt.Sprintf("dashboards:edit:%s", dash.ID) || p == fmt.Sprintf("dashboards:share:%s", dash.ID) {
			continue
		}
		filtered = append(filtered, p)
	}
	if _, err := c.UpdateRole(role.Name, &ic.Role{
		Description: role.Description,
		Permissions: filtered,
		ReadOnly:    role.ReadOnly,
	}); err != nil {
		t.Fatalf("UpdateRole (remove perms) error: %v", err)
	}
}
