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
				// Разные версии GL чувствительны к синтаксису DSL. Используем минимальный, совместимый source.
				Config: testAccProviderConfig() + `
resource "graylog_pipeline" "p" {
  title       = "tf_acc"
  description = "Acceptance pipeline"
  source = <<-EOT
pipeline "tf_acc"
stage 0 match either
end
  EOT
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_pipeline.p", "id"),
					testAccCheckLiveResourceExists("graylog_pipeline.p", "pipeline"),
					resource.TestCheckResourceAttr("graylog_pipeline.p", "title", "tf_acc"),
				),
			},
			{
				ResourceName:      "graylog_pipeline.p",
				ImportState:       true,
				ImportStateVerify: true,
				// На разных версиях возможны различия форматирования исходника
				ImportStateVerifyIgnore: []string{"source", "title"},
			},
		},
	})
}
