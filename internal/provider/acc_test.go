//go:build acceptance

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ProtoV6 provider factory map for acceptance tests
var testAccProtoV6ProviderFactories = map[string]func() (any, error){
	"graylog": func() (any, error) { return providerserver.NewProtocol6WithError(New()) },
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
	return `
provider "graylog" {
  url   = env("URL")
  token = env("TOKEN")
}
`
}
