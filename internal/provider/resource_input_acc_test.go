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

  configuration = {
    port = 1514
  }
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("graylog_input.syslog_udp", "id"),
					resource.TestCheckResourceAttr("graylog_input.syslog_udp", "title", "acc-syslog-udp"),
				),
			},
			{
				ResourceName:      "graylog_input.syslog_udp",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
