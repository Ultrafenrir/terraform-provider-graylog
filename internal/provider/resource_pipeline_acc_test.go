//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccPipeline_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_pipeline" "p" {
  title       = "acc-pipeline"
  description = "Acceptance pipeline"
  source = <<-EOT
    pipeline "acc"
    stage 0 match either
    rule "keep_all";

    rule "keep_all"
    when true
    then
      set_field("acc", true);
    end
  EOT
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_pipeline.p", "id"),
					resource.TestCheckResourceAttr("graylog_pipeline.p", "title", "acc-pipeline"),
				),
			},
			{
				ResourceName:      "graylog_pipeline.p",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
