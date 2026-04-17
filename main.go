package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	graylog "github.com/Ultrafenrir/terraform-provider-graylog/internal/provider"
)

var (
	// Set by goreleaser or ldflags at build time if needed
	version = "0.3.3"
)

func main() {
	if err := providerserver.Serve(
		context.Background(),
		graylog.New,
		providerserver.ServeOpts{
			Address: "registry.terraform.io/ultrafenrir/graylog",
			Debug:   false,
			// ProtocolVersions: []int{6}, // framework will negotiate by default
		},
	); err != nil {
		log.Fatal(err)
	}
}
