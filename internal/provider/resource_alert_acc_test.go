//go:build acceptance

package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func testAccAlertConfig(title, description string, priority int) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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
  title       = %q
  description = %q
  priority    = %d
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
`, title, description, priority)
}

func TestAccAlert_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Keep the default index set unchanged across Graylog 5, 6, and 7.
				// Creating the stream without index_set_id makes it use the system
				// writable index set and avoids cross-version plan drift.
				Config: testAccAlertConfig("acc-alert", "Acceptance alert", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_alert.a", "id"),
					testAccCheckLiveResourceExists("graylog_alert.a", "event_definition"),
					resource.TestCheckResourceAttr("graylog_alert.a", "title", "acc-alert"),
				),
			},
			{
				Config: testAccAlertConfig("acc-alert-updated", "Acceptance alert updated", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_alert.a", "title", "acc-alert-updated"),
					testAccCheckLiveResourceFields("graylog_alert.a", "alert", map[string]any{
						"title": "acc-alert-updated", "description": "Acceptance alert updated", "priority": 2, "alert": true,
					}),
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
