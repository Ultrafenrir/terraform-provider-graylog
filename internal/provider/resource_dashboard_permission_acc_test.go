//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestAccDashboardPermission_basic(t *testing.T) {
	// Capability pre-check: legacy dashboards CRUD может быть недоступен в GL 5.x/6.x образах
	{
		url := os.Getenv("URL")
		token := os.Getenv("TOKEN")
		if url == "" || token == "" {
			t.Skip("acceptance env is not configured: set URL and TOKEN env vars")
		}
		if _, err := base64.StdEncoding.DecodeString(token); err != nil {
			token = base64.StdEncoding.EncodeToString([]byte(token))
		}
		c := ic.New(url, token)
		if c.APIVersion == ic.APIV5 || c.APIVersion == ic.APIV6 {
			t.Skip("Dashboard CRUD is not supported by this Graylog version/image; skipping acceptance test")
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
resource "graylog_role" "r" {
  name        = "acc-dashperm-role"
  description = "Role for dashboard permission acc test"
}

resource "graylog_dashboard" "d" {
  title       = "acc-dashperm-dashboard"
  description = "Dashboard for permission acc test"
}

resource "graylog_dashboard_permission" "p" {
  role_name    = graylog_role.r.name
  dashboard_id = graylog_dashboard.d.id
  actions      = ["edit", "read"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_dashboard_permission.p", "id"),
					resource.TestCheckResourceAttr("graylog_dashboard_permission.p", "actions.#", "2"),
					// Actions are sorted alphabetically in state: edit, read
					resource.TestCheckResourceAttr("graylog_dashboard_permission.p", "actions.0", "edit"),
					resource.TestCheckResourceAttr("graylog_dashboard_permission.p", "actions.1", "read"),
				),
			},
			{
				ResourceName:      "graylog_dashboard_permission.p",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rm := s.RootModule()
					rRole, ok := rm.Resources["graylog_role.r"]
					if !ok {
						return "", fmt.Errorf("role resource not found in state")
					}
					rDash, ok := rm.Resources["graylog_dashboard.d"]
					if !ok {
						return "", fmt.Errorf("dashboard resource not found in state")
					}
					roleName := rRole.Primary.Attributes["name"]
					dashID := rDash.Primary.Attributes["id"]
					if roleName == "" || dashID == "" {
						return "", fmt.Errorf("missing role name or dashboard id in state")
					}
					return roleName + "/" + dashID, nil
				},
			},
		},
	})
}
