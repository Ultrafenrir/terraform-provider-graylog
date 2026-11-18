//go:build acceptance

package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	tfprotov6 "github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ProtoV6 provider factory map for acceptance tests
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"graylog": providerserver.NewProtocol6WithError(New()),
}

// testAccCheckLiveResourceFields verifies mutable fields through a separate
// API client after an update. State-only assertions cannot detect a provider
// that writes the plan to state without changing Graylog.
func testAccCheckLiveResourceFields(resourceName, kind string, want map[string]any) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok || rs.Primary == nil || rs.Primary.ID == "" {
			return fmt.Errorf("resource %s is missing an ID in Terraform state", resourceName)
		}
		c := testAccClient()
		got := map[string]any{}
		switch kind {
		case "alert":
			v, err := c.GetEventDefinition(rs.Primary.ID)
			if err != nil {
				return err
			}
			got = map[string]any{"title": v.Title, "description": v.Description, "priority": v.Priority, "alert": v.Alert}
		case "dashboard":
			v, err := c.GetDashboard(rs.Primary.ID)
			if err != nil {
				return err
			}
			got = map[string]any{"title": v.Title, "description": v.Description}
		case "event_notification":
			v, err := c.GetEventNotification(rs.Primary.ID)
			if err != nil {
				return err
			}
			got = map[string]any{"title": v.Title, "description": v.Description, "config": v.Config}
		case "output":
			v, err := c.GetOutput(rs.Primary.ID)
			if err != nil {
				return err
			}
			got = map[string]any{"title": v.Title, "configuration": v.Configuration}
		case "lookup_cache":
			v, err := c.GetLookupCache(rs.Primary.ID)
			if err != nil {
				return err
			}
			var cfg map[string]any
			if err := json.Unmarshal(v.Config, &cfg); err != nil {
				return err
			}
			got = map[string]any{"name": v.Name, "title": v.Title, "description": v.Description, "config": cfg}
		case "lookup_adapter":
			v, err := c.GetLookupAdapter(rs.Primary.ID)
			if err != nil {
				return err
			}
			var cfg map[string]any
			if err := json.Unmarshal(v.Config, &cfg); err != nil {
				return err
			}
			got = map[string]any{"name": v.Name, "title": v.Title, "description": v.Description, "config": cfg}
		default:
			return fmt.Errorf("unsupported live field check kind %q", kind)
		}

		for key, expected := range want {
			actual, exists := got[key]
			if !exists {
				return fmt.Errorf("live %s %q has no field %q", kind, rs.Primary.ID, key)
			}
			if expectedMap, ok := expected.(map[string]any); ok {
				actualMap, ok := actual.(map[string]any)
				if !ok {
					return fmt.Errorf("live %s field %q is not an object: %#v", kind, key, actual)
				}
				for nestedKey, nestedExpected := range expectedMap {
					if !reflect.DeepEqual(actualMap[nestedKey], nestedExpected) {
						return fmt.Errorf("live %s field %s.%s = %#v, want %#v", kind, key, nestedKey, actualMap[nestedKey], nestedExpected)
					}
				}
				continue
			}
			if !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("live %s field %q = %#v, want %#v", kind, key, actual, expected)
			}
		}
		return nil
	}
}

func testAccCheckLiveScopedPermissionsExact(roleResource, targetResource, scope string, actions ...string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		roleState := state.RootModule().Resources[roleResource]
		targetState := state.RootModule().Resources[targetResource]
		if roleState == nil || roleState.Primary == nil || targetState == nil || targetState.Primary == nil {
			return fmt.Errorf("missing role %s or target %s from Terraform state", roleResource, targetResource)
		}
		role, err := testAccClient().GetRole(roleState.Primary.Attributes["name"])
		if err != nil {
			return err
		}
		prefix, suffix := scope+":", ":"+targetState.Primary.ID
		got := map[string]bool{}
		for _, permission := range role.Permissions {
			if strings.HasPrefix(permission, prefix) && strings.HasSuffix(permission, suffix) {
				got[strings.TrimSuffix(strings.TrimPrefix(permission, prefix), suffix)] = true
			}
		}
		if len(got) != len(actions) {
			return fmt.Errorf("live scoped actions = %v, want %v", got, actions)
		}
		for _, action := range actions {
			if !got[action] {
				return fmt.Errorf("live scoped actions = %v, missing %q", got, action)
			}
		}
		return nil
	}
}

func testAccCheckLiveClusterConfig(resourceName, expected string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs := state.RootModule().Resources[resourceName]
		if rs == nil || rs.Primary == nil {
			return fmt.Errorf("resource %s is missing", resourceName)
		}
		raw, err := testAccClient().GetClusterConfig(rs.Primary.ID)
		if err != nil {
			return err
		}
		got, err := ProjectAndCanonicalizeJSON(string(raw), expected)
		if err != nil {
			return err
		}
		want, err := CanonicalizeJSONFromString(expected)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("live cluster config = %s, want %s", got, want)
		}
		return nil
	}
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("URL"); v == "" {
		t.Fatalf("URL must be set for acceptance tests")
	}
	if v := os.Getenv("TOKEN"); v == "" {
		t.Fatalf("TOKEN must be set for acceptance tests")
	}
}

// testAccCheckLiveResourceExists deliberately bypasses Terraform state and the
// resource implementation after apply. It reads the created object through a
// separate Graylog client, so a provider that merely returns a plausible ID or
// copies the plan into state cannot make an acceptance test pass.
func testAccCheckLiveResourceExists(resourceName, kind string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("resource %s is missing from Terraform state", resourceName)
		}
		id := resourceState.Primary.ID
		if id == "" {
			return fmt.Errorf("resource %s has an empty ID", resourceName)
		}

		c := testAccClient()

		var err error
		switch kind {
		case "auth_backend":
			_, err = c.GetAuthBackend(id)
		case "cluster_config":
			_, err = c.GetClusterConfig(id)
		case "dashboard":
			_, err = c.GetDashboard(id)
		case "event_definition":
			_, err = c.GetEventDefinition(id)
		case "event_notification":
			_, err = c.GetEventNotification(id)
		case "index_set":
			_, err = c.GetIndexSet(id)
		case "input":
			_, err = c.GetInput(id)
		case "lookup_adapter":
			_, err = c.GetLookupAdapter(id)
		case "lookup_cache":
			_, err = c.GetLookupCache(id)
		case "lookup_table":
			_, err = c.GetLookupTable(id)
		case "output":
			_, err = c.GetOutput(id)
		case "pipeline":
			_, err = c.GetPipeline(id)
		case "pipeline_rule":
			_, err = c.GetPipelineRule(id)
		case "role":
			_, err = c.GetRole(id)
		case "stream":
			_, err = c.GetStream(id)
		case "user":
			_, err = c.GetUser(id)
		default:
			return fmt.Errorf("unsupported live resource check kind %q", kind)
		}
		if err != nil {
			return fmt.Errorf("resource %s (%s, id %q) was not readable from live Graylog API: %w", resourceName, kind, id, err)
		}
		return nil
	}
}

// testAccCheckLiveScopedPermissions verifies effect resources which are stored
// as scoped strings on a Graylog role rather than as standalone API objects.
func testAccCheckLiveScopedPermissions(roleResource, targetResource, scope string, actions ...string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		roleState, roleOK := state.RootModule().Resources[roleResource]
		targetState, targetOK := state.RootModule().Resources[targetResource]
		if !roleOK || roleState.Primary == nil || !targetOK || targetState.Primary == nil {
			return fmt.Errorf("missing role %s or target %s from Terraform state", roleResource, targetResource)
		}
		roleName := roleState.Primary.Attributes["name"]
		targetID := targetState.Primary.ID
		if roleName == "" || targetID == "" {
			return fmt.Errorf("empty role name or target ID for %s permissions", scope)
		}

		role, err := testAccClient().GetRole(roleName)
		if err != nil {
			return fmt.Errorf("read role %q from live Graylog API: %w", roleName, err)
		}
		present := make(map[string]bool, len(role.Permissions))
		for _, permission := range role.Permissions {
			present[permission] = true
		}
		for _, action := range actions {
			want := fmt.Sprintf("%s:%s:%s", scope, action, targetID)
			if !present[want] {
				return fmt.Errorf("live role %q does not contain permission %q; got %v", roleName, want, role.Permissions)
			}
		}
		return nil
	}
}

func testAccCheckLiveStreamOutputBinding(streamResource, outputResource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		streamState, streamOK := state.RootModule().Resources[streamResource]
		outputState, outputOK := state.RootModule().Resources[outputResource]
		if !streamOK || streamState.Primary == nil || !outputOK || outputState.Primary == nil {
			return fmt.Errorf("missing stream %s or output %s from Terraform state", streamResource, outputResource)
		}
		streamID, outputID := streamState.Primary.ID, outputState.Primary.ID
		outputs, err := testAccClient().ListStreamOutputs(streamID)
		if err != nil {
			return fmt.Errorf("read outputs for stream %q from live Graylog API: %w", streamID, err)
		}
		for _, output := range outputs {
			if output.ID == outputID {
				return nil
			}
		}
		return fmt.Errorf("output %q is not attached to live stream %q", outputID, streamID)
	}
}

// testAccCheckLiveStreamIndexSet verifies the relationship through Graylog's
// stream API instead of trusting the dependency edge or Terraform state.
func testAccCheckLiveStreamIndexSet(streamResource, indexSetResource string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		streamState, streamOK := state.RootModule().Resources[streamResource]
		indexSetState, indexSetOK := state.RootModule().Resources[indexSetResource]
		if !streamOK || streamState.Primary == nil || !indexSetOK || indexSetState.Primary == nil {
			return fmt.Errorf("missing stream %s or index set %s from Terraform state", streamResource, indexSetResource)
		}
		stream, err := testAccClient().GetStream(streamState.Primary.ID)
		if err != nil {
			return fmt.Errorf("read stream %q from live Graylog API: %w", streamState.Primary.ID, err)
		}
		if stream.IndexSetID != indexSetState.Primary.ID {
			return fmt.Errorf("live stream %q points to index set %q, want %q", stream.ID, stream.IndexSetID, indexSetState.Primary.ID)
		}
		return nil
	}
}

func testAccCheckLiveSnapshotRepository(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok || resourceState.Primary == nil {
			return fmt.Errorf("resource %s is missing from Terraform state", resourceName)
		}
		name := resourceState.Primary.Attributes["name"]
		if name == "" {
			name = resourceState.Primary.ID
		}
		c := testAccClient()
		c.OSBaseURL = os.Getenv("OPENSEARCH_URL")
		if c.OSBaseURL == "" {
			c.OSBaseURL = "http://127.0.0.1:9200"
		}
		if _, _, err := c.OSGetSnapshotRepository(name); err != nil {
			return fmt.Errorf("snapshot repository %q was not readable from live OpenSearch API: %w", name, err)
		}
		return nil
	}
}

func testAccClient() *client.Client {
	token := os.Getenv("TOKEN")
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}
	return client.New(os.Getenv("URL"), token)
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
