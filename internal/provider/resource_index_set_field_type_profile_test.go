package provider

import (
	"context"
	"testing"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFieldTypeProfileReadState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prior  types.String
		remote string
		want   types.String
	}{
		{"omitted empty", types.StringNull(), "", types.StringValue("")},
		{"explicit empty", types.StringValue(""), "", types.StringValue("")},
		{"remote description", types.StringNull(), "changed", types.StringValue("changed")},
		{"external clear", types.StringValue("previous"), "", types.StringValue("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := indexSetFieldTypeProfileModel{Description: tc.prior}
			applyFieldTypeProfileReadState(context.Background(), &data, &client.IndexSetFieldTypeProfile{ID: "id", Name: "profile", Description: tc.remote})
			if !data.Description.Equal(tc.want) {
				t.Fatalf("description: want %v got %v", tc.want, data.Description)
			}
			if data.CustomFieldMappings.IsNull() || len(data.CustomFieldMappings.Elements()) != 0 {
				t.Fatal("empty mappings must be an empty map")
			}
		})
	}
}

func TestFieldTypeProfilePayloadOrdering(t *testing.T) {
	ctx := context.Background()
	mappings, diags := types.MapValueFrom(ctx, types.StringType, map[string]string{"z_count": "long", "a.client.ip": "ip"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	data := indexSetFieldTypeProfileModel{Name: types.StringValue("profile"), Description: types.StringNull(), CustomFieldMappings: mappings}
	profile, err := profilePayload(ctx, &data)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Description != "" || len(profile.CustomFieldMappings) != 2 || profile.CustomFieldMappings[0].Field != "a.client.ip" || profile.CustomFieldMappings[1].Type != "long" {
		t.Fatalf("unexpected payload %#v", profile)
	}
}

func TestIndexSetBindingReadState(t *testing.T) {
	remote := "remote-profile"
	for _, tc := range []struct {
		name   string
		prior  types.String
		remote *string
		want   types.String
	}{
		{"unmanaged", types.StringNull(), &remote, types.StringNull()},
		{"managed switch", types.StringValue("old-profile"), &remote, types.StringValue(remote)},
		{"managed detached", types.StringValue("old-profile"), nil, types.StringValue("")},
		{"explicit detach", types.StringValue(""), nil, types.StringValue("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := indexSetModel{FieldTypeProfile: tc.prior}
			applyIndexSetReadState(context.Background(), &data, &client.IndexSet{FieldTypeProfile: tc.remote})
			if !data.FieldTypeProfile.Equal(tc.want) {
				t.Fatalf("binding: want %v got %v", tc.want, data.FieldTypeProfile)
			}
		})
	}
}

func TestIndexSetBindingSchemaCompatible(t *testing.T) {
	var resp resource.SchemaResponse
	(&indexSetResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	attr := resp.Schema.Attributes["field_type_profile"]
	if !attr.IsOptional() || attr.IsComputed() || attr.IsRequired() {
		t.Fatal("binding must be optional without adoption of unmanaged server values")
	}
	if resp.Schema.Version != 4 {
		t.Fatal("new optional attributes must retain compatible state version")
	}
	for _, value := range []types.String{types.StringNull(), types.StringUnknown()} {
		if indexSetProfileBinding(value) != nil {
			t.Fatal("unset and unknown bindings cannot become explicit detach")
		}
	}
	empty := indexSetProfileBinding(types.StringValue(""))
	if empty == nil || *empty != "" {
		t.Fatal("explicit empty must detach")
	}
}
