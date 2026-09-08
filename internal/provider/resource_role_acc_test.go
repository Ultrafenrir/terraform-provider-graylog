//go:build acceptance

package provider

import (
	"regexp"
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

// A managed role exposes the same identifier without needing the data source.
func TestAccRole_exposesRoleID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_role" "acc" {
  name        = "tf-acc-role-id"
  description = "created by the test suite"
  permissions = ["streams:read"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					// id stays the name for backward compatibility...
					resource.TestCheckResourceAttr("graylog_role.acc", "id", "tf-acc-role-id"),
					// ...and role_id is the identifier the APIs want.
					resource.TestMatchResourceAttr("graylog_role.acc", "role_id", regexp.MustCompile(`^[0-9a-f]{24}$`)),
				),
			},
			{
				Config: testAccProviderConfig() + `
resource "graylog_role" "acc" {
  name        = "tf-acc-role-id"
  description = "created by the test suite"
  permissions = ["streams:read"]
}
`,
				PlanOnly: true,
			},
		},
	})
}
