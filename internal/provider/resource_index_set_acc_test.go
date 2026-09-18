//go:build acceptance

package provider

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func testAccCheckIndexSetWriteReady(id string) error {
	baseURL := os.Getenv("URL")
	token := os.Getenv("TOKEN")
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	status, err := client.New(baseURL, token).GetIndexSetDeflectorStatus(id)
	if err != nil {
		return fmt.Errorf("read index set %s deflector status: %w", id, err)
	}
	if !status.IsUp || status.CurrentTarget == "" {
		return fmt.Errorf("index set %s is not write-ready: is_up=%t current_target=%q", id, status.IsUp, status.CurrentTarget)
	}
	return nil
}

func TestAccIndexSet_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Test with rotation/retention blocks (modern syntax)
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-main-index"
  index_prefix       = "acc-main"
  description        = "Managed by acceptance"
  shards             = 1
  replicas           = 0
  field_type_refresh_interval         = 5000
  index_optimization_disabled         = false
  index_optimization_max_num_segments = 1
  default            = false

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "20000000"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "20"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					testAccCheckLiveResourceExists("graylog_index_set.test", "index_set"),
					resource.TestCheckResourceAttrWith("graylog_index_set.test", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-main-index"),
					// index_analyzer is Computed-only (not user-configurable); just confirm
					// Graylog's own value gets populated into state.
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "index_analyzer"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "rotation.class", "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "retention.class", "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"),
				),
			},
			{
				ResourceName:      "graylog_index_set.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccIndexSet_parallelCreation reproduces the common module pattern where
// one graylog_index_set resource uses for_each and Terraform creates all index
// sets concurrently. A single-resource test cannot expose coordination bugs in
// deflector initialization.
func TestAccIndexSet_parallelCreation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + testAccParallelIndexSetsConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.payments", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.payments", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.payments", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.payments", "graylog_index_set.payments"),
					resource.TestCheckResourceAttrSet("graylog_index_set.orders", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.orders", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.orders", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.orders", "graylog_index_set.orders"),
					resource.TestCheckResourceAttrSet("graylog_index_set.refunds", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.refunds", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.refunds", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.refunds", "graylog_index_set.refunds"),
					resource.TestCheckResourceAttrSet("graylog_index_set.invoices", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.invoices", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.invoices", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.invoices", "graylog_index_set.invoices"),
					resource.TestCheckResourceAttrSet("graylog_index_set.settlements", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.settlements", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.settlements", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.settlements", "graylog_index_set.settlements"),
					resource.TestCheckResourceAttrSet("graylog_index_set.transfers", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.transfers", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.transfers", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.transfers", "graylog_index_set.transfers"),
					resource.TestCheckResourceAttrSet("graylog_index_set.balances", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.balances", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.balances", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.balances", "graylog_index_set.balances"),
					resource.TestCheckResourceAttrSet("graylog_index_set.ledger", "id"),
					resource.TestCheckResourceAttrWith("graylog_index_set.ledger", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttrSet("graylog_stream.ledger", "id"),
					testAccCheckLiveStreamIndexSet("graylog_stream.ledger", "graylog_index_set.ledger"),
				),
			},
		},
	})
}

func testAccParallelIndexSetsConfig() string {
	config := ""
	for _, name := range []string{"payments", "orders", "refunds", "invoices", "settlements", "transfers", "balances", "ledger"} {
		config += fmt.Sprintf(`
resource "graylog_index_set" %q {
  title        = "acc-parallel-%s"
  index_prefix = "acc-parallel-%s"
  shards       = 1
  replicas     = 0

  timeouts = {
    create = "2m"
  }
}

resource "graylog_stream" %q {
  title        = "acc-parallel-%s"
  description  = "stream paired with acc-parallel-%s"
  index_set_id = graylog_index_set.%s.id

  rule {
    field = "source"
    type  = 1
    value = "acc-%s"
  }
}
`, name, name, name, name, name, name, name, name)
	}
	return config
}

func TestAccIndexSet_update(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-update-index"
  index_prefix       = "acc-update"
  description        = "Initial description"
  shards             = 1
  replicas           = 0
  field_type_refresh_interval         = 5000
  index_optimization_disabled         = false
  index_optimization_max_num_segments = 1
  default            = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-update-index"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "description", "Initial description"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "shards", "1"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "replicas", "0"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "field_type_refresh_interval", "5000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_disabled", "false"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_max_num_segments", "1"),
				),
			},
			{
				// index_prefix is left unchanged here (immutable, forces replace — see
				// TestAccIndexSet_immutableFieldsForceReplacement). Every other field, including
				// shards, is safe to change in place — the index set stores data and must never
				// be recreated just because shards/replicas/etc. changed.
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-update-index-modified"
  index_prefix       = "acc-update"
  description        = "Updated description"
  shards             = 2
  replicas           = 1
  field_type_refresh_interval         = 6000
  index_optimization_disabled         = true
  index_optimization_max_num_segments = 2
  default            = false
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_index_set.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-update-index-modified"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "shards", "2"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "replicas", "1"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "field_type_refresh_interval", "6000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_disabled", "true"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_max_num_segments", "2"),
				),
			},
			{
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-update-index-modified"
  index_prefix       = "acc-update"
  description        = ""
  shards             = 3
  replicas           = 2
  field_type_refresh_interval         = 7000
  index_optimization_disabled         = false
  index_optimization_max_num_segments = 3
  default            = false
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_index_set.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "description", ""),
					resource.TestCheckResourceAttr("graylog_index_set.test", "shards", "3"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "replicas", "2"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "field_type_refresh_interval", "7000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_disabled", "false"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_max_num_segments", "3"),
				),
			},
			{
				// A no-op plan after several updates must show zero changes — guards against the
				// Optional+Computed-without-UseStateForUnknown bug where any unrelated change
				// (or even just a refresh) could make untouched fields look like they need
				// recomputing.
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-update-index-modified"
  index_prefix       = "acc-update"
  description        = ""
  shards             = 3
  replicas           = 2
  field_type_refresh_interval         = 7000
  index_optimization_disabled         = false
  index_optimization_max_num_segments = 3
  default            = false
}
`,
				PlanOnly: true,
			},
		},
	})
}

// TestAccIndexSet_immutableFieldsForceReplacement verifies that ONLY index_prefix is immutable
// (forces replacement) — the underlying bug this guards against: index_prefix changes used to be
// silently dropped by UpdateIndexSet (permanent diff that never converged), and separately,
// shards/index_analyzer were incorrectly marked RequiresReplace even though Graylog only applies
// them to indices rotated in the future, never to already-written (data-bearing) ones. An index
// set holds real data — recreating it destroys that data, so nothing except renaming
// (index_prefix) may ever force a replace.
func TestAccIndexSet_immutableFieldsForceReplacement(t *testing.T) {
	baseConfig := func(prefix string, shards int) string {
		return testAccProviderConfig() + `
resource "graylog_index_set" "immutable" {
  title        = "acc-immutable-index"
  index_prefix = "` + prefix + `"
  shards       = ` + strconv.Itoa(shards) + `
  replicas     = 0
}
`
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: baseConfig("acc-immutable", 1),
				Check:  resource.TestCheckResourceAttrSet("graylog_index_set.immutable", "id"),
			},
			{
				// Changing shards must NOT force replacement — it's a safe in-place update.
				Config: baseConfig("acc-immutable", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_index_set.immutable", plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				// Changing index_prefix must force replacement. The new prefix must
				// not share a prefix with the old one in either direction: Graylog
				// validates prefix conflicts with startsWith (and the old set/indices
				// can still linger server-side while the replace is in flight), so
				// "acc-immutable-renamed" gets rejected with 400 on some versions.
				Config: baseConfig("acc-replaced-immutable", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("graylog_index_set.immutable", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccIndexSet_rotationRetentionConfig(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create with minimal rotation/retention config
				// API will return extra fields (type, max_rotation_period, etc)
				// Provider should NOT include them in state
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title        = "acc-config-filter"
  description  = "Test config filter"
  index_prefix = "acc-cfg-filter"
  shards       = 1
  replicas     = 0

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "1000000"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "3"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "rotation.config.max_docs_per_index", "1000000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "retention.config.max_number_of_indices", "3"),
					// These fields should NOT appear in state (API returns them but we filter)
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "rotation.config.type"),
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "rotation.config.max_rotation_period"),
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "rotation.config.rotate_empty_index_set"),
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "retention.config.type"),
				),
			},
			{
				// Update retention config - should not show extra fields appearing/disappearing
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title        = "acc-config-filter"
  description  = "Test config filter"
  index_prefix = "acc-cfg-filter"
  shards       = 1
  replicas     = 0

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "1000000"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "4"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("graylog_index_set.test", "retention.config.max_number_of_indices", "4"),
					// Still should not have extra fields
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "rotation.config.type"),
					resource.TestCheckNoResourceAttr("graylog_index_set.test", "retention.config.type"),
				),
			},
		},
	})
}

func TestAccIndexSet_timeBasedSizeOptimizingRotation(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "time_size_optimizing" {
  title        = "acc-time-size-optimizing"
  index_prefix = "acc-tso"
  shards       = 1
  replicas     = 0

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.TimeBasedSizeOptimizingStrategy"
    config = {
      index_lifetime_min = "P30D"
      index_lifetime_max = "P40D"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "20"
    }
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.time_size_optimizing", "id"),
					testAccCheckLiveResourceExists("graylog_index_set.time_size_optimizing", "index_set"),
					resource.TestCheckResourceAttrWith("graylog_index_set.time_size_optimizing", "id", testAccCheckIndexSetWriteReady),
					resource.TestCheckResourceAttr("graylog_index_set.time_size_optimizing", "rotation.class", "org.graylog2.indexer.rotation.strategies.TimeBasedSizeOptimizingStrategy"),
					resource.TestCheckResourceAttr("graylog_index_set.time_size_optimizing", "rotation.config.index_lifetime_min", "P30D"),
					resource.TestCheckResourceAttr("graylog_index_set.time_size_optimizing", "rotation.config.index_lifetime_max", "P40D"),
					resource.TestCheckNoResourceAttr("graylog_index_set.time_size_optimizing", "rotation.config.type"),
				),
			},
		},
	})
}
