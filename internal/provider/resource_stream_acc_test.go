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
				ExpectNonEmptyPlan: true,
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
