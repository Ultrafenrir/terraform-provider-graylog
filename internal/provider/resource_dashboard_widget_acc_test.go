//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDashboardWidget_basic(t *testing.T) {
	// Capability pre-check: creating classic dashboards may be unsupported (405) on some versions/images
	{
		url := os.Getenv("URL")
		token := os.Getenv("TOKEN")
		if url == "" || token == "" {
			t.Skip("acceptance env is not configured: set URL and TOKEN env vars")
		}
		if _, err := base64.StdEncoding.DecodeString(token); err != nil {
			token = base64.StdEncoding.EncodeToString([]byte(token))
		}
		c := ic.New(url, token)
		if c.APIVersion == ic.APIV5 || c.APIVersion == ic.APIV6 {
			t.Skip("Dashboard CRUD is not supported by this Graylog version/image; skipping acceptance test")
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_dashboard" "d" {
  title       = "acc-dash"
  description = "Acceptance dash"
}

resource "graylog_dashboard_widget" "w" {
  dashboard_id = graylog_dashboard.d.id
  type         = "SEARCH_RESULT_COUNT"
  description  = "acc widget"
  cache_time   = 1
  configuration = jsonencode({
    timerange = { type = "relative", range = 300 }
    query     = "*"
    stream_id = null
  })
}
`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_dashboard_widget.w", "id"),
					resource.TestCheckResourceAttr("graylog_dashboard_widget.w", "type", "SEARCH_RESULT_COUNT"),
				),
			},
			{
				ResourceName:      "graylog_dashboard_widget.w",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
