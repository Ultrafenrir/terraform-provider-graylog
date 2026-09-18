//go:build acceptance

package provider

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLDAPGroupMembers_basic(t *testing.T) {
	if os.Getenv("ENABLE_LDAP_ACC") == "" {
		t.Skip("LDAP group members acc test disabled; set ENABLE_LDAP_ACC=1 to enable")
	}
	waitLDAP := func(addr string, attempts int, delay time.Duration) {
		for i := 0; i < attempts; i++ {
			c, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				_ = c.Close()
				return
			}
			time.Sleep(delay)
		}
	}
	ldapURL := os.Getenv("LDAP_URL")
	if ldapURL == "" {
		ldapURL = "ldap://127.0.0.1:1389"
	}
	ldapAddr := strings.TrimPrefix(ldapURL, "ldap://")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); waitLDAP(ldapAddr, 120, time.Second) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
data "graylog_ldap_group_members" "devops" {
  url            = %q
  bind_dn        = "cn=admin,dc=example,dc=org"
  bind_password  = "admin"
  base_dn        = "dc=example,dc=org"
  group_name     = "devops"
}
`, ldapURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.graylog_ldap_group_members.devops", "id"),
					// Expect two members from LDIF (alice, bob)
					resource.TestCheckResourceAttr("data.graylog_ldap_group_members.devops", "members.#", "2"),
				),
			},
		},
	})
}
