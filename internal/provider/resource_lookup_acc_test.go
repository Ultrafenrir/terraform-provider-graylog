//go:build acceptance

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// A complete lookup stack, the shape a GeoIP or CSV enrichment setup takes:
// the table references the cache and adapter, so Terraform orders creation
// and destruction correctly without any explicit depends_on.
func testAccLookupStack(tableTitle string) string {
	return testAccProviderConfig() + `
resource "graylog_lookup_cache" "c" {
  name  = "tf-acc-cache"
  title = "TF acc cache"

  config_json = jsonencode({
    type                     = "guava_cache"
    max_size                 = 1000
    expire_after_access      = 60
    expire_after_access_unit = "SECONDS"
    expire_after_write       = 0
  })
}

resource "graylog_lookup_adapter" "a" {
  name  = "tf-acc-adapter"
  title = "TF acc adapter"

  config_json = jsonencode({
    type                    = "csvfile"
    path                    = "/etc/graylog/server/tf-acc.csv"
    separator               = ","
    quotechar               = "\""
    key_column              = "k"
    value_column            = "v"
    check_interval          = 60
    case_insensitive_lookup = false
  })
}

resource "graylog_lookup_table" "t" {
  name            = "tf-acc-table"
  title           = "` + tableTitle + `"
  cache_id        = graylog_lookup_cache.c.id
  data_adapter_id = graylog_lookup_adapter.a.id
}
`
}

// The regression guard that matters: Graylog enriches a stored cache
// configuration with the defaults of its type, so a Read that adopted the
// echo wholesale would show a diff on every plan forever.
func TestAccLookupStack_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLookupStack("TF acc table"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_lookup_cache.c", "id"),
					resource.TestCheckResourceAttrSet("graylog_lookup_adapter.a", "id"),
					resource.TestCheckResourceAttrSet("graylog_lookup_table.t", "id"),
					testAccCheckLiveResourceExists("graylog_lookup_cache.c", "lookup_cache"),
					testAccCheckLiveResourceExists("graylog_lookup_adapter.a", "lookup_adapter"),
					testAccCheckLiveResourceExists("graylog_lookup_table.t", "lookup_table"),
					resource.TestCheckResourceAttr("graylog_lookup_table.t", "name", "tf-acc-table"),
					// Left unset in the configuration, so the server value is
					// adopted rather than fought over.
					resource.TestCheckResourceAttr("graylog_lookup_table.t", "default_single_value_type", "NULL"),
					resource.TestCheckResourceAttr("graylog_lookup_table.t", "default_multi_value_type", "NULL"),
					// The table must point at the objects Terraform created.
					resource.TestCheckResourceAttrPair(
						"graylog_lookup_table.t", "cache_id", "graylog_lookup_cache.c", "id"),
					resource.TestCheckResourceAttrPair(
						"graylog_lookup_table.t", "data_adapter_id", "graylog_lookup_adapter.a", "id"),
				),
			},
			{
				// Nothing changed, so nothing may be planned.
				Config: testAccLookupStack("TF acc table"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// An in-place update must not disturb the other two.
				Config: testAccLookupStack("TF acc table renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_lookup_table.t", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("graylog_lookup_cache.c", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("graylog_lookup_adapter.a", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.TestCheckResourceAttr("graylog_lookup_table.t", "title", "TF acc table renamed"),
			},
			{
				// Re-plan after the update must be empty too.
				Config: testAccLookupStack("TF acc table renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "graylog_lookup_table.t",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				ResourceName:      "graylog_lookup_cache.c",
				ImportState:       true,
				ImportStateVerify: true,
				// Import has no prior document to project against, so it
				// stores the enriched server configuration.
				ImportStateVerifyIgnore: []string{"config_json", "timeouts"},
			},
		},
	})
}
