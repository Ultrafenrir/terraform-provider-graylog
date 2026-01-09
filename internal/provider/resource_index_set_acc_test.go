//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccIndexSet_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Graylog sets several defaults; define them explicitly to avoid plan drift.
				// Still, API may return nested strategy blocks on refresh; allow non-empty plan.
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-main-index"
  index_prefix       = "acc-main"
  description        = "Managed by acceptance"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 1
  replicas                            = 0
  index_analyzer                      = "standard"
  field_type_refresh_interval         = 5000
  index_optimization_disabled         = false
  index_optimization_max_num_segments = 1
  // Не делаем этот набор индексов default, чтобы не конфликтовать с системным
  // default на Graylog 5.x, где переключение может быть запрещено.
  default                             = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-main-index"),
				),
			},
			{
				ResourceName:      "graylog_index_set.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"rotation.%", "rotation.class", "rotation.config.%", "rotation.config.type", "rotation.config.max_docs_per_index",
					"retention.%", "retention.class", "retention.config.%", "retention.config.type", "retention.config.max_number_of_indices",
					"rotation_strategy", "retention_strategy",
				},
			},
		},
	})
}
