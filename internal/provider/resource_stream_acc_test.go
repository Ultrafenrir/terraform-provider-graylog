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
				// Для совместимости GL 5/6/7 не трогаем default index set вовсе,
				// и не привязываем stream к кастомному index set. Так он
				// будет создан на системном writable (default) index set,
				// что исключает конфликты и дрейф плана.
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "acc-stream"
  description  = "Acceptance test stream"
  index_set_id = data.graylog_index_set_default.this.id

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
					resource.TestCheckResourceAttr("graylog_stream.s", "description", "Acceptance test stream"),
				),
			},
			{
				ResourceName:      "graylog_stream.s",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"disabled",
					"rule.0.description",
					"rule.0.inverted",
				},
			},
		},
	})
}

func TestAccStream_update(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "test" {
  title                           = "acc-stream-update"
  description                     = "Initial stream"
  index_set_id                    = data.graylog_index_set_default.this.id
  disabled                        = false
  remove_matches_from_default_stream = false

  rule {
    field       = "source"
    type        = 1
    value       = "initial"
    inverted    = false
    description = "initial rule"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_stream.test", "id"),
					resource.TestCheckResourceAttr("graylog_stream.test", "title", "acc-stream-update"),
					resource.TestCheckResourceAttr("graylog_stream.test", "description", "Initial stream"),
					resource.TestCheckResourceAttr("graylog_stream.test", "disabled", "false"),
					resource.TestCheckResourceAttr("graylog_stream.test", "remove_matches_from_default_stream", "false"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.field", "source"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.value", "initial"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.inverted", "false"),
				),
			},
			{
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "test" {
  title                           = "acc-stream-updated"
  description                     = "Updated stream"
  index_set_id                    = data.graylog_index_set_default.this.id
  disabled                        = true
  remove_matches_from_default_stream = true

  rule {
    field       = "message"
    type        = 2
    value       = "updated"
    inverted    = true
    description = "updated rule"
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_stream.test", "id"),
					resource.TestCheckResourceAttr("graylog_stream.test", "title", "acc-stream-updated"),
					resource.TestCheckResourceAttr("graylog_stream.test", "description", "Updated stream"),
					resource.TestCheckResourceAttr("graylog_stream.test", "disabled", "true"),
					resource.TestCheckResourceAttr("graylog_stream.test", "remove_matches_from_default_stream", "true"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.field", "message"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.value", "updated"),
					resource.TestCheckResourceAttr("graylog_stream.test", "rule.0.inverted", "true"),
				),
			},
		},
	})
}

