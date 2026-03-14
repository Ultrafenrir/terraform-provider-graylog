package provider

import (
	"context"
	"fmt"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// ensureFeature returns diagnostics error if the feature is not supported by current Graylog instance.
// versionHint is optional; pass empty string to omit.
func ensureFeature(ctx context.Context, c *client.Client, supported bool, featureName, versionHint string) (d diag.Diagnostics) {
	if supported {
		return d
	}
	hint := ""
	if versionHint != "" {
		hint = ": " + versionHint
	}
	d.AddError(
		fmt.Sprintf("Feature '%s' is not available", featureName),
		fmt.Sprintf("This Graylog instance does not expose required APIs for '%s'%s.", featureName, hint),
	)
	return d
}
