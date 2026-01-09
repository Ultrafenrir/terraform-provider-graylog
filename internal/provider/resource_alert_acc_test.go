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
				// Для совместимости GL 5/6/7 не меняем default index set и не создаём
				// кастомный index set для stream. Создаём stream без index_set_id,
				// чтобы он использовал системный writable (default) index set.
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "acc-stream-alert"
  description  = "Acceptance stream for alert"
  index_set_id = data.graylog_index_set_default.this.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_alert" "a" {
  title       = "acc-alert"
  description = "Acceptance alert"
  priority    = 1
  alert       = true

  config = jsonencode({
    type  = "aggregation-v1"
    query = "*"
    streams = [graylog_stream.s.id]
    group_by = []
    search_within_ms  = 60000
    execute_every_ms  = 60000
    conditions = {
      expression = {
        expr = ">"
        left = { expr = "number-ref", ref = "count" }
        right = { expr = "number", value = 0 }
      }
    }
    series = [{ id = "count", function = "count" }]
  })
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
				// Разные версии GL расширяют config при отдаче (query_parameters, поля серии и т.п.).
				// Игнорируем config при сравнении после импорта, чтобы избежать дрейфа.
				ImportStateVerifyIgnore: []string{
					"config",
				},
			},
		},
	})
}
