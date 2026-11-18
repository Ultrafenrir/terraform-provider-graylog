//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func testAccPipelineStack(updated bool) string {
	description := "Initial pipeline resources"
	connection := "graylog_pipeline.primary.id"
	if updated {
		description = "Updated pipeline resources"
		connection = "graylog_pipeline.primary.id, graylog_pipeline.secondary.id"
	}

	return testAccProviderConfig() + `
data "graylog_index_set_default" "pipeline" {}

resource "graylog_stream" "pipeline" {
  title        = "tf-acc-pipeline-stream"
  description  = "Pipeline connection acceptance stream"
  index_set_id = data.graylog_index_set_default.pipeline.id

  rule {
    field = "source"
    type  = 1
    value = "tf-acc"
  }
}

resource "graylog_pipeline_rule" "rule" {
  description = "` + description + `"
  source = <<-EOT
rule "tf_acc_rule"
when
  true
then
end
  EOT
}

resource "graylog_pipeline" "primary" {
  title       = "tf_acc_primary"
  description = "` + description + `"
  source = <<-EOT
pipeline "tf_acc_primary"
stage 0 match either
  rule "tf_acc_rule";
end
  EOT

  depends_on = [graylog_pipeline_rule.rule]
}

resource "graylog_pipeline" "secondary" {
  title       = "tf_acc_secondary"
  description = "Secondary pipeline"
  source = <<-EOT
pipeline "tf_acc_secondary"
stage 0 match either
end
  EOT
}

resource "graylog_pipeline_connection" "connection" {
  stream_id    = graylog_stream.pipeline.id
  pipeline_ids = [` + connection + `]
}
`
}

func TestAccPipeline_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Graylog versions differ in DSL parsing details. Use the smallest compatible source.
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
				// Source formatting can differ between Graylog versions.
				ImportStateVerifyIgnore: []string{"source", "title"},
			},
		},
	})
}

// Covers the three separately registered pipeline resources end to end. The
// connection owns the complete list for a stream, so growing the set exercises
// its Update path without disrupting the connected pipeline or rule.
func TestAccPipelineStack_lifecycle(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPipelineStack(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_pipeline_rule.rule", "id"),
					resource.TestCheckResourceAttrSet("graylog_pipeline.primary", "id"),
					resource.TestCheckResourceAttrSet("graylog_pipeline_connection.connection", "id"),
					testAccCheckLiveResourceExists("graylog_pipeline_rule.rule", "pipeline_rule"),
					testAccCheckLiveResourceExists("graylog_pipeline.primary", "pipeline"),
					resource.TestCheckResourceAttr("graylog_pipeline_connection.connection", "pipeline_ids.#", "1"),
				),
			},
			{
				Config: testAccPipelineStack(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_pipeline_rule.rule", "description", "Updated pipeline resources"),
					resource.TestCheckResourceAttr("graylog_pipeline.primary", "description", "Updated pipeline resources"),
					resource.TestCheckResourceAttr("graylog_pipeline_connection.connection", "pipeline_ids.#", "2"),
				),
			},
			{
				ResourceName:      "graylog_pipeline_rule.rule",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            "graylog_pipeline_connection.connection",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"id"},
			},
		},
	})
}
