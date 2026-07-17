//go:build acceptance

package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccInput_syslogUDP(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_input" "syslog_udp" {
  title  = "acc-syslog-udp"
  type   = "org.graylog2.inputs.syslog.udp.SyslogUDPInput"
  global = true

  configuration = jsonencode({
    bind_address = "0.0.0.0"
    port         = 1514
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_input.syslog_udp", "id"),
					resource.TestCheckResourceAttr("graylog_input.syslog_udp", "title", "acc-syslog-udp"),
				),
			},
			{
				ResourceName:            "graylog_input.syslog_udp",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"configuration", "node"},
			},
		},
	})
}

// TestAccInput_kafkaRaw covers the Kafka input's real field names (legacy_mode, bootstrap_server,
// topic_filter, custom_properties, ...) verified against a live Graylog 6.0.14 instance — see
// docs/resources/graylog_input.md. The update step in particular guards a critical bug found via
// live testing: Update() previously read `id` from the plan (Unknown for a Computed attribute)
// instead of prior state, sending every update to "PUT /system/inputs/" with no ID at all,
// which Graylog rejected with 405. `bootstrap_server` intentionally points at an unreachable
// host — Graylog accepts the Create/Update HTTP request synchronously and only asynchronously
// (and separately) tries and fails to connect the actual Kafka consumer, so this doesn't affect
// resource CRUD.
func TestAccInput_kafkaRaw(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "graylog_input" "kafka_raw" {
  title  = "acc-kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode        = false
    bootstrap_server   = "kafka.invalid:9092"
    topic_filter       = "^acc-logs-.*$"
    fetch_min_bytes    = 1
    fetch_wait_max     = 100
    threads            = 2
    group_id           = "acc-kafka-raw"
    offset_reset       = "largest"
    custom_properties  = "security.protocol=PLAINTEXT"
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_input.kafka_raw", "id"),
					resource.TestCheckResourceAttr("graylog_input.kafka_raw", "title", "acc-kafka-raw"),
				),
			},
			{
				// Changing a value (fetch_min_bytes) and re-applying exercises Update() — this
				// is what caught the plan-vs-state ID bug in live testing.
				Config: testAccProviderConfig() + `
resource "graylog_input" "kafka_raw" {
  title  = "acc-kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode        = false
    bootstrap_server   = "kafka.invalid:9092"
    topic_filter       = "^acc-logs-.*$"
    fetch_min_bytes    = 5
    fetch_wait_max     = 100
    threads            = 2
    group_id           = "acc-kafka-raw"
    offset_reset       = "largest"
    custom_properties  = "security.protocol=PLAINTEXT"
  })
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_input.kafka_raw", "id"),
				),
			},
			{
				ResourceName:            "graylog_input.kafka_raw",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"node"},
			},
		},
	})
}
