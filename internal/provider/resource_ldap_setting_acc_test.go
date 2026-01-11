//go:build acceptance

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLDAPSetting_basic(t *testing.T) {
	// LDAP settings endpoint недоступен в большинстве OSS-образов Graylog (Enterprise feature).
	// Чтобы не красить общий прогон acceptance, пропускаем этот тест в CI окружении.
	t.Skip("LDAP settings not available in OSS Graylog images; skipping acceptance test")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ExpectNonEmptyPlan: true,
				Config: testAccProviderConfig() + `
resource "graylog_ldap_setting" "this" {
  enabled = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_ldap_setting.this", "id"),
				),
			},
			{
				ResourceName:            "graylog_ldap_setting.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"system_password"},
			},
		},
	})
}
