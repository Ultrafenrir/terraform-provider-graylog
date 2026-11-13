package client

import (
	"encoding/json"
	"fmt"
	"net/url"
)

const fieldTypeProfilesPath = "/api/system/indices/index_sets/profiles"

// CustomFieldMapping uses Graylog's logical type names, such as string and geo-point.
type CustomFieldMapping struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

type IndexSetFieldTypeProfile struct {
	ID                  string               `json:"id,omitempty"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	CustomFieldMappings []CustomFieldMapping `json:"custom_field_mappings"`
	// Usage information is returned only by GET and must not be written back.
	IndexSetIDs []string `json:"-"`
}

func decodeFieldTypeProfile(raw []byte) (*IndexSetFieldTypeProfile, error) {
	var profile IndexSetFieldTypeProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("decode field type profile: %w", err)
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("field type profile response has no id")
	}
	var usages struct {
		IndexSetIDs []string `json:"index_set_ids"`
	}
	if err := json.Unmarshal(raw, &usages); err != nil {
		return nil, err
	}
	profile.IndexSetIDs = usages.IndexSetIDs
	return &profile, nil
}

func (c *Client) CreateIndexSetFieldTypeProfile(profile *IndexSetFieldTypeProfile) (*IndexSetFieldTypeProfile, error) {
	if c.APIVersion == APIV5 {
		return nil, fmt.Errorf("index set field type profiles require Graylog 6 or later")
	}
	// Create must not reuse a caller's ID or emit null for an empty mapping set.
	body := *profile
	body.ID = ""
	if body.CustomFieldMappings == nil {
		body.CustomFieldMappings = []CustomFieldMapping{}
	}
	raw, err := c.doRequest("POST", fieldTypeProfilesPath, &body)
	if err != nil {
		return nil, err
	}
	return decodeFieldTypeProfile(raw)
}

func (c *Client) GetIndexSetFieldTypeProfile(id string) (*IndexSetFieldTypeProfile, error) {
	if c.APIVersion == APIV5 {
		return nil, fmt.Errorf("index set field type profiles require Graylog 6 or later")
	}
	raw, err := c.doRequest("GET", fieldTypeProfilesPath+"/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	return decodeFieldTypeProfile(raw)
}

func (c *Client) UpdateIndexSetFieldTypeProfile(id string, profile *IndexSetFieldTypeProfile) error {
	if c.APIVersion == APIV5 {
		return fmt.Errorf("index set field type profiles require Graylog 6 or later")
	}
	// Graylog's PUT is on the collection and returns 204, with the ID in the body.
	body := *profile
	body.ID = id
	if body.CustomFieldMappings == nil {
		body.CustomFieldMappings = []CustomFieldMapping{}
	}
	_, err := c.doRequest("PUT", fieldTypeProfilesPath, &body)
	return err
}

func (c *Client) DeleteIndexSetFieldTypeProfile(id string) error {
	if c.APIVersion == APIV5 {
		return fmt.Errorf("index set field type profiles require Graylog 6 or later")
	}
	_, err := c.doRequest("DELETE", fieldTypeProfilesPath+"/"+url.PathEscape(id), nil)
	return err
}
