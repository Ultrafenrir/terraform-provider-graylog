//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccDashboard_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_dashboard" "d" {
  title       = "acc-dashboard"
  description = "Acceptance dashboard"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_dashboard.d", "id"),
					resource.TestCheckResourceAttr("graylog_dashboard.d", "title", "acc-dashboard"),
				),
			},
			{
				ResourceName:      "graylog_dashboard.d",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
