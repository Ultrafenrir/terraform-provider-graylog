//go:build acceptance

package provider

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func testAccAuthBackend(title, password string) string {
	return testAccProviderConfig() + `
resource "graylog_auth_backend" "ldap" {
  title                = "` + title + `"
  system_user_password = "` + password + `"

  config_json = jsonencode({
    type                     = "ldap"
    servers                  = [{ host = "openldap", port = 389 }]
    transport_security       = "none"
    verify_certificates      = false
    system_user_dn           = "cn=admin,dc=example,dc=org"
    user_full_name_attribute = "cn"
    user_name_attribute      = "uid"
    user_search_base         = "dc=example,dc=org"
    user_search_pattern      = "(&(uid={0})(objectClass=person))"
    user_unique_id_attribute = "entryUUID"
  })
}

resource "graylog_auth_backend_activation" "active" {
  backend_id = graylog_auth_backend.ldap.id
}
`
}

// The regression guard that matters here: system_user_password only ever
// exists in state, because Graylog reports it as {"is_set": true}. A Read
// that adopted the echo would either erase the secret or fight the
// configuration on every plan.
func TestAccAuthBackend_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAuthBackend("tf-acc-ldap", "admin"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_auth_backend.ldap", "id"),
					resource.TestCheckResourceAttr("graylog_auth_backend.ldap", "title", "tf-acc-ldap"),
					resource.TestCheckResourceAttr("graylog_auth_backend.ldap", "system_user_password", "admin"),
					resource.TestCheckResourceAttr("graylog_auth_backend_activation.active", "id", "active"),
					resource.TestCheckResourceAttrPair(
						"graylog_auth_backend_activation.active", "backend_id",
						"graylog_auth_backend.ldap", "id"),
				),
			},
			{
				// Nothing changed, so nothing may be planned — in particular
				// the write-only password must not manufacture a diff.
				Config: testAccAuthBackend("tf-acc-ldap", "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// An in-place update must leave the activation alone.
				Config: testAccAuthBackend("tf-acc-ldap renamed", "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_auth_backend.ldap", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("graylog_auth_backend_activation.active", plancheck.ResourceActionNoop),
					},
				},
			},
			{
				Config: testAccAuthBackend("tf-acc-ldap renamed", "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// Rotating the password is applied like any other change, even though the new
// value can never be read back to confirm it. The plan afterwards must still
// be empty: state is the only record of the secret, so a refresh that tried
// to verify it would diff forever.
func TestAccAuthBackend_passwordRotation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAuthBackend("tf-acc-rotation", "admin"),
			},
			{
				Config: testAccAuthBackend("tf-acc-rotation", "rotated-secret"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_auth_backend.ldap", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"graylog_auth_backend.ldap", "system_user_password", "rotated-secret"),
			},
			{
				Config: testAccAuthBackend("tf-acc-rotation", "rotated-secret"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// Import has to adopt the server's configuration. Reading it into an empty
// config_json used to leave the attribute empty, because an empty document
// became an empty projection mask and both sides compared as {}.
func TestAccAuthBackend_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAuthBackend("tf-acc-import", "admin"),
			},
			{
				ResourceName:      "graylog_auth_backend.ldap",
				ImportState:       true,
				ImportStateVerify: true,
				// The password is not returned by Graylog, and the imported
				// document carries the server's own additions, so neither
				// matches the prior state verbatim.
				ImportStateVerifyIgnore: []string{"system_user_password", "config_json", "timeouts"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported instance, got %d", len(states))
					}
					config := states[0].Attributes["config_json"]
					if config == "" || config == "{}" {
						return fmt.Errorf("import left config_json empty: %q", config)
					}
					if strings.Contains(config, "system_user_password") || strings.Contains(config, "is_set") {
						return fmt.Errorf("the write-only field leaked into the imported configuration: %s", config)
					}
					if !strings.Contains(config, "user_search_base") {
						return fmt.Errorf("imported configuration is missing server values: %s", config)
					}
					return nil
				},
			},
			{
				ResourceName:            "graylog_auth_backend_activation.active",
				ImportState:             true,
				ImportStateId:           "active",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}

// An empty password is a configuration mistake, not a way to say "keep the
// stored one" — omitting the attribute is. Rejecting it at plan time stops an
// empty string from silently becoming keep-existing.
func TestAccAuthBackend_emptyPasswordRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAuthBackend("tf-acc-empty-password", ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)system_user_password.*at least 1`),
			},
		},
	})
}

// The reason the role id is exposed at all: default_roles only works with
// ids. Graylog stores names without complaint and then refuses every login,
// so both sources of an id have to reach the backend end to end and the
// result has to survive a refresh without a diff.
func TestAccAuthBackend_defaultRolesByID(t *testing.T) {
	config := testAccProviderConfig() + `
data "graylog_role" "reader" {
  name = "Reader"
}

resource "graylog_role" "acc_viewers" {
  name        = "tf-acc-backend-viewers"
  description = "created by the test suite"
  permissions = ["dashboards:read", "streams:read"]
}

resource "graylog_auth_backend" "with_roles" {
  title                = "tf-acc-ldap-default-roles"
  system_user_password = "admin"
  default_roles        = [data.graylog_role.reader.id, graylog_role.acc_viewers.role_id]

  config_json = jsonencode({
    type                     = "ldap"
    servers                  = [{ host = "openldap", port = 389 }]
    transport_security       = "none"
    verify_certificates      = false
    system_user_dn           = "cn=admin,dc=example,dc=org"
    user_full_name_attribute = "cn"
    user_name_attribute      = "uid"
    user_search_base         = "dc=example,dc=org"
    user_search_pattern      = "(&(uid={0})(objectClass=person))"
    user_unique_id_attribute = "entryUUID"
  })
}
`
	objectID := regexp.MustCompile(`^[0-9a-f]{24}$`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_auth_backend.with_roles", "default_roles.#", "2"),
					resource.TestCheckTypeSetElemAttrPair(
						"graylog_auth_backend.with_roles", "default_roles.*",
						"data.graylog_role.reader", "id"),
					resource.TestCheckTypeSetElemAttrPair(
						"graylog_auth_backend.with_roles", "default_roles.*",
						"graylog_role.acc_viewers", "role_id"),
					// Every element is an id, never a name.
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["graylog_auth_backend.with_roles"]
						if !ok {
							return errors.New("graylog_auth_backend.with_roles is not in state")
						}
						for k, v := range rs.Primary.Attributes {
							if strings.HasPrefix(k, "default_roles.") && k != "default_roles.#" && !objectID.MatchString(v) {
								return fmt.Errorf("%s = %q is not a role id", k, v)
							}
						}
						return nil
					},
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
