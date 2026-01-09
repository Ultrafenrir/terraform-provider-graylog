//go:build acceptance

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	tfprotov6 "github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// ProtoV6 provider factory map for acceptance tests
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"graylog": providerserver.NewProtocol6WithError(New()),
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("URL"); v == "" {
		t.Fatalf("URL must be set for acceptance tests")
	}
	if v := os.Getenv("TOKEN"); v == "" {
		t.Fatalf("TOKEN must be set for acceptance tests")
	}
}

// Common provider configuration used in acceptance tests
func testAccProviderConfig() string {
	url := os.Getenv("URL")
	if url == "" {
		url = "http://127.0.0.1:9000/api"
	}
	token := os.Getenv("TOKEN")
	return fmt.Sprintf(`
provider "graylog" {
  url   = "%s"
  token = "%s"
}
`, url, token)
}
