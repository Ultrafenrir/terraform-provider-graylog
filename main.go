package main

import (
	"context"
	"log"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	if err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{ Address: "registry.terraform.io/ultrafenrir/graylog" }); err != nil {
		log.Fatal(err)
	}
}
