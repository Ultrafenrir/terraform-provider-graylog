//go:build acceptance

package provider

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func profileAccClient(t *testing.T) *client.Client {
	t.Helper()
	testAccPreCheck(t)
	api := client.New(os.Getenv("URL"), os.Getenv("TOKEN"))
	if api.APIVersion != client.APIV7 {
		t.Skip("field type profiles acceptance contract is verified on Graylog 7.2.2")
	}
	return api
}

func TestAccIndexSetFieldTypeProfile_lifecycle(t *testing.T) {
	var api *client.Client
	var id string
	config := func(name, description, mappings string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_index_set_field_type_profile" "test" {
 name = %q
 %s
 custom_field_mappings = %s
}
`, name, description, mappings)
	}
	initial := config("Acceptance profile", "", `{ "client.ip" = "ip", "response_code" = "long", duration = "double", label = "string", text = "string_fts", recorded_at = "date", active = "boolean", raw = "binary", location = "geo-point" }`)
	updated := config("Updated acceptance profile", `description = "A description"`, `{ "client.ip" = "string", "duration" = "double" }`)
	cleared := config("Updated acceptance profile", `description = ""`, `{}`)
	check := func(name, description string, count int) resource.TestCheckFunc {
		return resource.TestCheckResourceAttrWith("graylog_index_set_field_type_profile.test", "id", func(value string) error {
			id = value
			p, err := api.GetIndexSetFieldTypeProfile(value)
			if err != nil {
				return err
			}
			wantMappings := map[string]string{}
			if count == 9 {
				wantMappings = map[string]string{"client.ip": "ip", "response_code": "long", "duration": "double", "label": "string", "text": "string_fts", "recorded_at": "date", "active": "boolean", "raw": "binary", "location": "geo-point"}
			} else if count == 2 {
				wantMappings = map[string]string{"client.ip": "string", "duration": "double"}
			}
			gotMappings := map[string]string{}
			for _, mapping := range p.CustomFieldMappings {
				gotMappings[mapping.Field] = mapping.Type
			}
			if p.Name != name || p.Description != description || !reflect.DeepEqual(gotMappings, wantMappings) {
				return fmt.Errorf("unexpected live profile %#v", p)
			}
			return nil
		})
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { api = profileAccClient(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			for _, rs := range s.RootModule().Resources {
				if rs.Type == "graylog_index_set_field_type_profile" {
					if _, err := api.GetIndexSetFieldTypeProfile(rs.Primary.ID); !errors.Is(err, client.ErrNotFound) {
						return fmt.Errorf("profile still exists or read failed: %v", err)
					}
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: initial, Check: resource.ComposeTestCheckFunc(check("Acceptance profile", "", 9), resource.TestCheckResourceAttr("graylog_index_set_field_type_profile.test", "custom_field_mappings.client.ip", "ip"))},
			{ResourceName: "graylog_index_set_field_type_profile.test", ImportState: true, ImportStateVerify: true},
			{Config: initial, PlanOnly: true},
			{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("graylog_index_set_field_type_profile.test", plancheck.ResourceActionUpdate)}}, Check: check("Updated acceptance profile", "A description", 2)},
			{Config: updated, PlanOnly: true},
			{PreConfig: func() {
				err := api.UpdateIndexSetFieldTypeProfile(id, &client.IndexSetFieldTypeProfile{Name: "External drift", Description: "Changed", CustomFieldMappings: []client.CustomFieldMapping{{Field: "client.ip", Type: "boolean"}}})
				if err != nil {
					t.Fatal(err)
				}
			}, Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("graylog_index_set_field_type_profile.test", plancheck.ResourceActionUpdate)}}, Check: check("Updated acceptance profile", "A description", 2)},
			{Config: cleared, Check: check("Updated acceptance profile", "", 0)},
			{Config: cleared, PlanOnly: true},
			{ResourceName: "graylog_index_set_field_type_profile.test", ImportState: true, ImportStateVerify: true},
			{PreConfig: func() {
				if err := api.DeleteIndexSetFieldTypeProfile(id); err != nil {
					t.Fatal(err)
				}
			}, Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("graylog_index_set_field_type_profile.test", plancheck.ResourceActionCreate)}}, Check: check("Updated acceptance profile", "", 0)},
			{Config: cleared, PlanOnly: true},
		},
	})
}

func TestAccIndexSetFieldTypeProfile_binding(t *testing.T) {
	var api *client.Client
	var setID, alphaID, betaID, initialTarget string
	prefix := fmt.Sprintf("acc_profile_%d", time.Now().UnixNano())
	config := func(binding, title string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "graylog_index_set_field_type_profile" "alpha" {
 name = "Acceptance alpha"
 custom_field_mappings = { "client.ip" = "ip" }
}
resource "graylog_index_set_field_type_profile" "beta" {
 name = "Acceptance beta"
 custom_field_mappings = { "client.ip" = "string" }
}
resource "graylog_index_set" "test" {
 title = %q
 index_prefix = %q
 shards = 1
 replicas = 0
 rotation {
  class = "org.graylog2.indexer.rotation.strategies.TimeBasedRotationStrategy"
  config = { rotation_period = "P1D" }
 }
 retention {
  class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
  config = { max_number_of_indices = "30" }
 }
 %s
}
`, title, prefix, binding)
	}
	alpha := `field_type_profile = graylog_index_set_field_type_profile.alpha.id`
	beta := `field_type_profile = graylog_index_set_field_type_profile.beta.id`
	check := func(want string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrWith("graylog_index_set_field_type_profile.alpha", "id", func(id string) error { alphaID = id; return nil }),
			resource.TestCheckResourceAttrWith("graylog_index_set_field_type_profile.beta", "id", func(id string) error { betaID = id; return nil }),
			resource.TestCheckResourceAttrWith("graylog_index_set.test", "id", func(id string) error {
				setID = id
				is, err := api.GetIndexSet(id)
				if err != nil {
					return err
				}
				expected := ""
				if want == "alpha" {
					expected = alphaID
				}
				if want == "beta" {
					expected = betaID
				}
				got := ""
				if is.FieldTypeProfile != nil {
					got = *is.FieldTypeProfile
				}
				if got != expected {
					return fmt.Errorf("live binding: want %q got %q", expected, got)
				}
				if is.RotationStrategyConfig["rotation_period"] != "P1D" || is.RetentionStrategyConfig["max_number_of_indices"] != float64(30) {
					return fmt.Errorf("profile update changed rotation/retention: %#v / %#v", is.RotationStrategyConfig, is.RetentionStrategyConfig)
				}
				status, err := api.GetIndexSetDeflectorStatus(id)
				if err != nil {
					return err
				}
				if initialTarget == "" {
					initialTarget = status.CurrentTarget
				} else if status.CurrentTarget != initialTarget {
					return fmt.Errorf("binding update rotated %s to %s", initialTarget, status.CurrentTarget)
				}
				return nil
			}),
		)
	}
	externalBind := func(profileID func() string) func() {
		return func() {
			is, err := api.GetIndexSet(setID)
			if err != nil {
				t.Fatal(err)
			}
			id := profileID()
			is.FieldTypeProfile = &id
			if _, err := api.UpdateIndexSet(setID, is); err != nil {
				t.Fatal(err)
			}
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { api = profileAccClient(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// The profile ID is unknown during planning on this first apply.
			{Config: config(alpha, "Attached alpha"), Check: resource.ComposeTestCheckFunc(check("alpha"), resource.TestCheckResourceAttrPair("graylog_index_set.test", "field_type_profile", "graylog_index_set_field_type_profile.alpha", "id"))},
			{ResourceName: "graylog_index_set.test", ImportState: true, ImportStateVerify: true},
			{Config: config(alpha, "Attached alpha"), PlanOnly: true},
			{Config: config(beta, "Switched beta"), Check: check("beta")},
			{Config: config(beta, "Switched beta"), PlanOnly: true},
			// Removing the attribute relinquishes management without detaching.
			{Config: config("", "Unmanaged beta"), Check: resource.ComposeTestCheckFunc(check("beta"), resource.TestCheckNoResourceAttr("graylog_index_set.test", "field_type_profile"))},
			// A server-side binding remains unmanaged through an unrelated PUT.
			{PreConfig: externalBind(func() string { return alphaID }), Config: config("", "Metadata changed, alpha unmanaged"), Check: check("alpha")},
			{Config: config(`field_type_profile = ""`, "Detached"), Check: check("")},
			{Config: config(`field_type_profile = ""`, "Detached"), PlanOnly: true},
			{Config: config(alpha, "Managed alpha"), Check: check("alpha")},
			{PreConfig: externalBind(func() string { return betaID }), Config: config(alpha, "Managed alpha"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("graylog_index_set.test", plancheck.ResourceActionUpdate)}}, Check: check("alpha")},
			{PreConfig: externalBind(func() string { return "" }), Config: config(alpha, "Managed alpha"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("graylog_index_set.test", plancheck.ResourceActionUpdate)}}, Check: check("alpha")},
			{Config: config(alpha, "Managed alpha"), PlanOnly: true},
		},
	})
}

func TestAccIndexSetFieldTypeProfile_deleteInUse(t *testing.T) {
	api := profileAccClient(t)
	profile, err := api.CreateIndexSetFieldTypeProfile(&client.IndexSetFieldTypeProfile{Name: "Delete in-use probe", CustomFieldMappings: []client.CustomFieldMapping{{Field: "client.ip", Type: "ip"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = api.DeleteIndexSetFieldTypeProfile(profile.ID) }()
	is, err := api.CreateIndexSet(&client.IndexSet{Title: "Delete in-use probe", IndexPrefix: fmt.Sprintf("acc_delete_profile_%d", time.Now().UnixNano()), Shards: 1, FieldTypeProfile: &profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := api.DeleteIndexSet(is.ID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()
	if err := api.DeleteIndexSetFieldTypeProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GetIndexSetFieldTypeProfile(profile.ID); !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("deleted profile: %v", err)
	}
	// Graylog intentionally removes all references, rather than rejecting in-use deletion.
	live, err := api.GetIndexSet(is.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.FieldTypeProfile != nil && *live.FieldTypeProfile != "" {
		t.Fatalf("Graylog did not detach deleted profile: %#v", live.FieldTypeProfile)
	}
	if err := api.DeleteIndexSetFieldTypeProfile(profile.ID); err != nil {
		t.Fatalf("delete must be idempotent: %v", err)
	}
}
