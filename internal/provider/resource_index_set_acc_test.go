//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccIndexSet_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-main-index"
  description        = "Managed by acceptance"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 1
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-main-index"),
				),
			},
			{
				ResourceName:      "graylog_index_set.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
