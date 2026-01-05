//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccStream_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "is" {
  title              = "acc-idx-stream"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 1
}

resource "graylog_stream" "s" {
  title        = "acc-stream"
  description  = "Acceptance test stream"
  index_set_id = graylog_index_set.is.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_stream.s", "id"),
					resource.TestCheckResourceAttr("graylog_stream.s", "title", "acc-stream"),
				),
			},
			{
				ResourceName:      "graylog_stream.s",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
