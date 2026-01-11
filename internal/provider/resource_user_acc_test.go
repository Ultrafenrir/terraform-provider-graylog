//go:build acceptance

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUser_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_user" "u" {
  username = "acc-user"
  full_name = "Acc User"
  email = "acc@example.com"
  roles = ["Reader"]
  password = "ChangeMe123!"
}
`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_user.u", "id"),
					resource.TestCheckResourceAttr("graylog_user.u", "username", "acc-user"),
				),
			},
			{
				ResourceName:            "graylog_user.u",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "timezone", "disabled"},
			},
		},
	})
}
