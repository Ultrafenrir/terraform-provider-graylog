//go:build acceptance

package provider

import (
	"errors"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The data source exists so that an id can reach an API that only accepts
// ids. Asserting the id is not the name is the whole point.
func TestAccRoleDataSource_builtIn(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "graylog_role" "reader" {
  name = "Reader"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.graylog_role.reader", "name", "Reader"),
					resource.TestCheckResourceAttr("data.graylog_role.reader", "read_only", "true"),
					resource.TestMatchResourceAttr("data.graylog_role.reader", "id", regexp.MustCompile(`^[0-9a-f]{24}$`)),
					resource.TestCheckResourceAttrWith("data.graylog_role.reader", "permissions.#", func(v string) error {
						if v == "0" {
							return errors.New("Reader should grant permissions, got none")
						}
						return nil
					}),
				),
			},
		},
	})
}

func TestAccRoleDataSource_missing(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "graylog_role" "missing" {
  name = "tf-acc-no-such-role"
}
`,
				ExpectError: regexp.MustCompile("Role not found"),
			},
		},
	})
}
