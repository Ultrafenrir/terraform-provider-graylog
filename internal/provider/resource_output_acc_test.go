//go:build acceptance

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOutput_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "acc-out-stream"
  description  = "Acceptance Output Stream"
  index_set_id = data.graylog_index_set_default.this.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_output" "gelf" {
  title = "to-local-gelf"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_output.gelf", "id"),
					resource.TestCheckResourceAttr("graylog_output.gelf", "title", "to-local-gelf"),
				),
			},
			{
				ResourceName:            "graylog_output.gelf",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"configuration", "streams.%", "streams"},
			},
		},
	})
}
