//go:build acceptance

package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccIndexSet_dataTiering(t *testing.T) {
	prefix := fmt.Sprintf("acc_tiering_%d", time.Now().UnixNano())
	var api *client.Client
	liveCheck := func(min, max string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttrWith("graylog_index_set.tiered", "id", func(id string) error {
			is, err := api.GetIndexSet(id)
			if err != nil {
				return err
			}
			var document map[string]any
			if err := json.Unmarshal(is.DataTiering, &document); err != nil {
				return err
			}
			if document["type"] != "hot_only" || document["index_lifetime_min"] != min || document["index_lifetime_max"] != max {
				return fmt.Errorf("unexpected live tiering configuration: %s", is.DataTiering)
			}
			return nil
		})
	}
	config := func(min, max string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_index_set" "tiered" {
  title = "Acceptance data tiering"
  index_prefix = %q
  shards = 1
  replicas = 0
  use_legacy_rotation = false
  data_tiering = jsonencode({
    type = "hot_only"
    index_lifetime_min = %q
    index_lifetime_max = %q
  })
}
`, prefix, min, max)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			token := os.Getenv("TOKEN")
			if _, err := base64.StdEncoding.DecodeString(token); err != nil {
				token = base64.StdEncoding.EncodeToString([]byte(token))
			}
			api = client.New(os.Getenv("URL"), token)
			if api.APIVersion == client.APIV5 {
				t.Skip("data tiering requires Graylog 6+")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("P30D", "P40D"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_index_set.tiered", "use_legacy_rotation", "false"),
					liveCheck("P30D", "P40D"),
				),
			},
			{
				ResourceName:      "graylog_index_set.tiered",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("P60D", "P70D"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("graylog_index_set.tiered", plancheck.ResourceActionUpdate),
				}},
				Check: liveCheck("P60D", "P70D"),
			},
			{
				Config:   config("P60D", "P70D"),
				PlanOnly: true,
			},
			{
				// Stop managing tiering and change metadata in the same apply.
				// The server must retain the complete policy after this PUT.
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_index_set" "tiered" {
  title = "Metadata changed, tiering unmanaged"
  index_prefix = %q
  shards = 1
  replicas = 0
}
`, prefix),
				Check: liveCheck("P60D", "P70D"),
			},
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_index_set" "tiered" {
  title = "Switch back to legacy"
  index_prefix = %q
  shards = 1
  replicas = 0
  use_legacy_rotation = true
  rotation {
    class = "org.graylog2.indexer.rotation.strategies.TimeBasedRotationStrategy"
    config = { rotation_period = "P1D" }
  }
  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = { max_number_of_indices = "30" }
  }
}
`, prefix),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("graylog_index_set.tiered", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttrWith("graylog_index_set.tiered", "id", func(id string) error {
					is, err := api.GetIndexSet(id)
					if err != nil {
						return err
					}
					if is.HasDataTiering() || is.RotationStrategyClass != "org.graylog2.indexer.rotation.strategies.TimeBasedRotationStrategy" {
						return fmt.Errorf("legacy rotation was not applied: tiering=%s rotation=%s", is.DataTiering, is.RotationStrategyClass)
					}
					return nil
				}),
			},
			{
				Config: config("P30D", "P40D"),
				Check:  liveCheck("P30D", "P40D"),
			},
		},
	})
}
