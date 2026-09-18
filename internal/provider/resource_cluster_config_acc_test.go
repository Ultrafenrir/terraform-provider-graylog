//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// discoverClusterConfigDocuments returns up to n classes that already hold a
// document, together with those documents, so the configuration under test is
// valid on whichever Graylog version the matrix is currently running. Which
// classes carry a document varies by version — SearchesClusterConfig is
// populated on 5.x and 7.x but empty on 6.x — so the caller states how many
// it needs and the test skips when the server cannot supply them.
func discoverClusterConfigDocuments(t *testing.T, n int) ([]string, []string) {
	t.Helper()
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	c := client.New(baseURL, token)

	var classes, docs []string
	for _, candidate := range clusterConfigProbeClasses {
		doc, err := c.GetClusterConfig(candidate)
		if err != nil {
			continue
		}
		classes = append(classes, candidate)
		docs = append(docs, string(doc))

		// Destroy removes the document and returns the class to the default
		// compiled into the server, which would otherwise starve later tests
		// of the documents they discover. Put each one back afterwards so the
		// suite stays order-independent.
		restoreClass, restoreDoc := candidate, doc
		t.Cleanup(func() {
			if _, err := c.UpdateClusterConfig(restoreClass, restoreDoc); err != nil {
				t.Logf("could not restore %s: %v", restoreClass, err)
			}
		})

		if len(classes) == n {
			return classes, docs
		}
	}
	t.Skipf("need %d probe classes holding a document, found %d of %v", n, len(classes), clusterConfigProbeClasses)
	return nil, nil
}

func testAccClusterConfig(class, doc string) string {
	// HCL would read ${...} inside a quoted string as an interpolation.
	escaped := strings.ReplaceAll(strconv.Quote(doc), "${", "$${")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_cluster_config" "c" {
  class       = %q
  config_json = %s
}
`, class, escaped)
}

// Writing a document and re-planning must produce no diff. This is the
// regression guard for the failure mode this provider has hit repeatedly:
// a Read that adopts the server echo wholesale and then fights the
// practitioner's configuration on every plan.
func TestAccClusterConfig_basic(t *testing.T) {
	classes, docs := discoverClusterConfigDocuments(t, 1)
	class, doc := classes[0], docs[0]
	config := testAccClusterConfig(class, doc)

	// Destroy removes the document, which returns the class to the default
	// compiled into the server. That is the intended semantic and is safe on
	// the throwaway instance the acceptance suite runs against.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLiveResourceExists("graylog_cluster_config.c", "cluster_config"),
					resource.TestCheckResourceAttr("graylog_cluster_config.c", "id", class),
					resource.TestCheckResourceAttr("graylog_cluster_config.c", "class", class),
					resource.TestCheckResourceAttrSet("graylog_cluster_config.c", "config_json"),
				),
			},
			{
				// Re-applying the same configuration must be a no-op.
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "graylog_cluster_config.c",
				ImportState:       true,
				ImportStateId:     class,
				ImportStateVerify: true,
				// Import canonicalizes the document, so the stored string can
				// differ from the practitioner's formatting even though the
				// two are semantically identical.
				ImportStateVerifyIgnore: []string{"config_json", "timeouts"},
			},
		},
	})
}

// Changing the class means a different document, so it must replace the
// resource rather than update it in place.
func TestAccClusterConfig_classForcesReplacement(t *testing.T) {
	classes, docs := discoverClusterConfigDocuments(t, 2)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccClusterConfig(classes[0], docs[0]),
				Check:  resource.TestCheckResourceAttr("graylog_cluster_config.c", "id", classes[0]),
			},
			{
				Config: testAccClusterConfig(classes[1], docs[1]),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_cluster_config.c", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("graylog_cluster_config.c", "id", classes[1]),
			},
		},
	})
}
