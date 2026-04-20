//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

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
  index_analyzer     = "standard"
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
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-main-index"),
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
  index_analyzer     = "standard"
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
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_analyzer", "standard"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "field_type_refresh_interval", "5000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_disabled", "false"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_max_num_segments", "1"),
				),
			},
			{
				// Test update - это должно использовать PUT без 405 ошибок
				Config: testAccProviderConfig() + `
resource "graylog_index_set" "test" {
  title              = "acc-update-index-modified"
  index_prefix       = "acc-update"
  description        = "Updated description"
  shards             = 2
  replicas           = 1
  index_analyzer     = "simple"
  field_type_refresh_interval         = 6000
  index_optimization_disabled         = true
  index_optimization_max_num_segments = 2
  default            = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_index_set.test", "id"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "title", "acc-update-index-modified"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "description", "Updated description"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "shards", "2"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "replicas", "1"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_analyzer", "simple"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "field_type_refresh_interval", "6000"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_disabled", "true"),
					resource.TestCheckResourceAttr("graylog_index_set.test", "index_optimization_max_num_segments", "2"),
				),
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
