//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccAlert_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_alert" "a" {
  title       = "acc-alert"
  description = "Acceptance alert"
  priority    = 1
  alert       = true

  config = {
    type  = "aggregation-v1"
    query = "*"
    series = [{ id = "count", function = "count()" }]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_alert.a", "id"),
					resource.TestCheckResourceAttr("graylog_alert.a", "title", "acc-alert"),
				),
			},
			{
				ResourceName:      "graylog_alert.a",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
