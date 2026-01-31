//go:build acceptance

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRole_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_role" "r" {
  name        = "tf-acc-role"
  description = "acc role"
  permissions = ["dashboards:read", "indices:read"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_role.r", "id"),
					resource.TestCheckResourceAttr("graylog_role.r", "name", "tf-acc-role"),
				),
			},
			{
				ResourceName:      "graylog_role.r",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
