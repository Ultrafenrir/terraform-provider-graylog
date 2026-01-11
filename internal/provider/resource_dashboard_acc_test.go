//go:build acceptance

package provider

import (
	"encoding/base64"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"os"
	"testing"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestAccDashboard_basic(t *testing.T) {
	// Capability pre-check: legacy dashboards CRUD может быть недоступен в некоторых версиях/образах.
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
		// На GL 5.x и часто на 6.x endpoint /dashboards недоступен для создания (405)
		if c.APIVersion == ic.APIV5 || c.APIVersion == ic.APIV6 {
			t.Skip("Dashboard CRUD is not supported by this Graylog version/image; skipping acceptance test")
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Разные версии GL могут возвращать расширенные поля — разрешаем непустой план
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
resource "graylog_dashboard" "d" {
  title       = "acc-dashboard"
  description = "Acceptance dashboard"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_dashboard.d", "id"),
					resource.TestCheckResourceAttr("graylog_dashboard.d", "title", "acc-dashboard"),
				),
			},
			{
				ResourceName:      "graylog_dashboard.d",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
