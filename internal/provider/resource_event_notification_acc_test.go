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

func TestAccEventNotification_basic(t *testing.T) {
	// Graylog 5 has different payload schema for notifications; skip until v5 mapping is implemented
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
		if c.APIVersion == ic.APIV5 {
			t.Skip("Event Notification payload differs on Graylog 5.x; skipping acceptance test for v5")
		}
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_event_notification" "n" {
  title = "acc-email"
  type  = "email"
  config = jsonencode({
    sender           = "noreply@example.com"
    subject          = "TF Acc"
    body_template    = "Test"
    user_recipients  = ["admin"]
    email_recipients = ["root@example.com"]
  })
}
`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_event_notification.n", "id"),
					resource.TestCheckResourceAttr("graylog_event_notification.n", "title", "acc-email"),
				),
			},
			{
				ResourceName:      "graylog_event_notification.n",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
