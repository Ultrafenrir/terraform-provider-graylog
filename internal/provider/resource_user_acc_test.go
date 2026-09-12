//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	ic "github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUser_basic(t *testing.T) {
	uname := fmt.Sprintf("acc-user-%d", time.Now().UnixNano())
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_user" "u" {
  username = "%s"
  full_name = "Acc User"
  email = "acc@example.com"
  roles = ["Reader"]
  password = "ChangeMe123!"
}
`, uname),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_user.u", "id"),
					resource.TestCheckResourceAttr("graylog_user.u", "username", uname),
				),
			},
			{
				ResourceName:            "graylog_user.u",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "timezone"},
			},
		},
	})
}

// The consumer scenario for directory-managed users: Terraform changes the
// roles while the password stays as it is, with disabled and
// session_timeout_ms omitted and read back from the server. The framework's
// post-apply plan check asserts an empty plan after every step.
func TestAccUser_rolesUpdateKeepsPassword(t *testing.T) {
	// The user update endpoint on Graylog 5 expects an ObjectId the client
	// does not resolve there (see TestIntegration_UserCRUD); skip on v5.
	{
		url := os.Getenv("URL")
		token := os.Getenv("TOKEN")
		if url == "" || token == "" {
			t.Skip("acceptance env is not configured: set URL and TOKEN env vars")
		}
		if _, err := base64.StdEncoding.DecodeString(token); err != nil {
			token = base64.StdEncoding.EncodeToString([]byte(token))
		}
		if ic.New(url, token).APIVersion == ic.APIV5 {
			t.Skip("user update is not supported on Graylog 5.x; skipping acceptance test for v5")
		}
	}
	uname := fmt.Sprintf("acc-user-pw-%d", time.Now().UnixNano())
	cfg := func(roles, extra string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_user" "u" {
  username  = "%s"
  full_name = "Acc User"
  email     = "acc-pw@example.com"
  roles     = %s
  %s
}
`, uname, roles, extra)
	}
	const password = `password = "ChangeMe123!"`
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// disabled and session_timeout_ms omitted: both are read back
				Config: cfg(`["Reader"]`, password),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_user.u", "roles.#", "1"),
					resource.TestCheckResourceAttr("graylog_user.u", "disabled", "false"),
					// unset in config: the server default is read back, never 0
					resource.TestCheckResourceAttrWith("graylog_user.u", "session_timeout_ms", func(v string) error {
						if v == "" || v == "0" {
							return fmt.Errorf("session_timeout_ms = %q, want the server default", v)
						}
						return nil
					}),
				),
			},
			{
				// roles change, password unchanged; disabled = false set
				// explicitly must plan clean against the state read back above
				Config: cfg(`["Reader", "Admin"]`, password+"\n  disabled = false"),
				Check:  resource.TestCheckResourceAttr("graylog_user.u", "roles.#", "2"),
			},
			{
				// disabled removed from the configuration is not a change
				Config:   cfg(`["Reader", "Admin"]`, password),
				PlanOnly: true,
			},
			{
				// password removed from the configuration is not a change
				Config:   cfg(`["Reader", "Admin"]`, ``),
				PlanOnly: true,
			},
			{
				// the same password re-added is not a change either
				Config:   cfg(`["Reader", "Admin"]`, password),
				PlanOnly: true,
			},
		},
	})
}
