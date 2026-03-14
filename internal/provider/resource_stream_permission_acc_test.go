//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
)

func TestAccStreamPermission_basic(t *testing.T) {
	// Capability/env pre-check
	{
		url := os.Getenv("URL")
		token := os.Getenv("TOKEN")
		if url == "" || token == "" {
			t.Skip("acceptance env is not configured: set URL and TOKEN env vars")
		}
		if _, err := base64.StdEncoding.DecodeString(token); err != nil {
			token = base64.StdEncoding.EncodeToString([]byte(token))
		}
		_ = ic.New(url, token) // just to validate basic client init
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "def" {}

resource "graylog_role" "r" {
  name        = "acc-streamperm-role"
  description = "Role for stream permission acc test"
  # Graylog 5.x requires non-null permissions array; use minimal safe perms
  permissions = ["dashboards:read", "indices:read"]
}

resource "graylog_stream" "s" {
  title        = "acc-streamperm-stream"
  description  = "Stream for permission acc test"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_stream_permission" "p" {
  role_name = graylog_role.r.name
  stream_id = graylog_stream.s.id
  actions   = ["edit", "read"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_stream_permission.p", "id"),
					resource.TestCheckResourceAttr("graylog_stream_permission.p", "actions.#", "2"),
				),
			},
			{
				// Update: narrow permissions to read-only
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
data "graylog_index_set_default" "def" {}

resource "graylog_role" "r" {
  name        = "acc-streamperm-role"
  description = "Role for stream permission acc test"
  permissions = ["dashboards:read", "indices:read"]
}

resource "graylog_stream" "s" {
  title        = "acc-streamperm-stream"
  description  = "Stream for permission acc test"
  index_set_id = data.graylog_index_set_default.def.id

  rule {
    field = "source"
    type  = 1
    value = "acc"
  }
}

resource "graylog_stream_permission" "p" {
  role_name = graylog_role.r.name
  stream_id = graylog_stream.s.id
  actions   = ["read"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_stream_permission.p", "actions.#", "1"),
					resource.TestCheckResourceAttr("graylog_stream_permission.p", "actions.0", "read"),
				),
			},
			{
				ResourceName:      "graylog_stream_permission.p",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rm := s.RootModule()
					rRole, ok := rm.Resources["graylog_role.r"]
					if !ok {
						return "", fmt.Errorf("role resource not found in state")
					}
					rStream, ok := rm.Resources["graylog_stream.s"]
					if !ok {
						return "", fmt.Errorf("stream resource not found in state")
					}
					roleName := rRole.Primary.Attributes["name"]
					streamID := rStream.Primary.Attributes["id"]
					if roleName == "" || streamID == "" {
						return "", fmt.Errorf("missing role name or stream id in state")
					}
					return roleName + "/" + streamID, nil
				},
			},
		},
	})
}
