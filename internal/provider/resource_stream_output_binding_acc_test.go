//go:build acceptance

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccStreamOutputBinding_diffAware(t *testing.T) {
	if os.Getenv("ENABLE_BINDING_ACC") == "" {
		t.Skip("Stream Output binding acceptancе test is disabled by default; set ENABLE_BINDING_ACC=1 to enable")
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create stream s1, s2; outputs o1, o2; bind s1<->o1
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "def" {}

resource "graylog_stream" "s1" {
  title        = "acc-bind-s1"
  description  = "Acceptance binding stream 1"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_stream" "s2" {
  title        = "acc-bind-s2"
  description  = "Acceptance binding stream 2"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_output" "o1" {
  title = "acc-bind-o1"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_output" "o2" {
  title = "acc-bind-o2"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_stream_output_binding" "b" {
  stream_id = graylog_stream.s1.id
  output_id = graylog_output.o1.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_stream_output_binding.b", "id"),
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "stream_id", "graylog_stream.s1", "id"),
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "output_id", "graylog_output.o1", "id"),
				),
			},
			{
				// Change only output (same stream): s1<->o2
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "def" {}

resource "graylog_stream" "s1" {
  title        = "acc-bind-s1"
  description  = "Acceptance binding stream 1"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_stream" "s2" {
  title        = "acc-bind-s2"
  description  = "Acceptance binding stream 2"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_output" "o1" {
  title = "acc-bind-o1"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_output" "o2" {
  title = "acc-bind-o2"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_stream_output_binding" "b" {
  stream_id = graylog_stream.s1.id
  output_id = graylog_output.o2.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "stream_id", "graylog_stream.s1", "id"),
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "output_id", "graylog_output.o2", "id"),
				),
			},
			{
				// Change stream (same output): s2<->o2
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "def" {}

resource "graylog_stream" "s1" {
  title        = "acc-bind-s1"
  description  = "Acceptance binding stream 1"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_stream" "s2" {
  title        = "acc-bind-s2"
  description  = "Acceptance binding stream 2"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_output" "o1" {
  title = "acc-bind-o1"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_output" "o2" {
  title = "acc-bind-o2"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
}

resource "graylog_stream_output_binding" "b" {
  stream_id = graylog_stream.s2.id
  output_id = graylog_output.o2.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "stream_id", "graylog_stream.s2", "id"),
					resource.TestCheckResourceAttrPair("graylog_stream_output_binding.b", "output_id", "graylog_output.o2", "id"),
				),
			},
		},
	})
}
