package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type APIVersion int

const (
	APIV5 APIVersion = iota
	APIV6
	APIV7
)

// ErrNotFound indicates a 404 resource not found error
var ErrNotFound = errors.New("resource not found")

type Client struct {
	BaseURL    string
	Token      string
	HTTP       *http.Client
	APIVersion APIVersion
	MaxRetries int
	RetryWait  time.Duration
}

func New(baseURL, token string) *Client {
	c := &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
		RetryWait:  time.Second,
	}
	c.detectVersion()
	return c
}

func (c *Client) detectVersion() {
	c.APIVersion = APIV5
	resp, err := c.HTTP.Get(c.BaseURL + "/system")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		if v := resp.Header.Get("X-Graylog-Version"); v != "" {
			if strings.HasPrefix(v, "7.") {
				c.APIVersion = APIV7
			} else if strings.HasPrefix(v, "6.") {
				c.APIVersion = APIV6
			}
		}
	}
}

// shouldRetry determines if a request should be retried based on status code
func (c *Client) shouldRetry(statusCode int) bool {
	// Retry on rate limiting and server errors
	return statusCode == 429 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

func (c *Client) doRequest(method, path string, body any) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		// Prepare request body for each attempt
		var buf io.Reader
		if bodyBytes != nil {
			buf = bytes.NewBuffer(bodyBytes)
		}

		req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.BaseURL, path), buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-By", "terraform-provider")
		req.Header.Set("Authorization", "Basic "+c.Token)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			// Network error - retry if attempts remain
			lastErr = err
			if attempt < c.MaxRetries {
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
				time.Sleep(waitTime)
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", c.MaxRetries+1, err)
		}
		defer resp.Body.Close()

		// 404 - resource not found, don't retry
		if resp.StatusCode == 404 {
			return nil, ErrNotFound
		}

		// Read response body
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Check if we should retry
		if resp.StatusCode >= 400 {
			if c.shouldRetry(resp.StatusCode) && attempt < c.MaxRetries {
				lastErr = fmt.Errorf("Graylog API error (status %d): %s", resp.StatusCode, string(b))
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
				time.Sleep(waitTime)
				continue
			}
			return nil, fmt.Errorf("Graylog API error (status %d): %s", resp.StatusCode, string(b))
		}

		// Success
		return b, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.MaxRetries+1, lastErr)
}

// Legacy lightweight rule used in initial implementation for embedding into Stream payload.
// Graylog actually manages stream rules via dedicated endpoints; use StreamRule below for full support.
type Rule struct{ Field, Type, Value string }

type Stream struct {
	ID           string `json:"id,omitempty"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
	IndexSetID   string `json:"index_set_id,omitempty"`
	MatchingType string `json:"matching_type,omitempty"`
	Rules        []Rule `json:"rules,omitempty"`
}

func (c *Client) CreateStream(s *Stream) (*Stream, error) {
	path := "/streams"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/streams"
	}
	resp, err := c.doRequest("POST", path, s)
	if err != nil {
		return nil, err
	}
	var out Stream
	// Graylog 5.x may return {"stream_id": "..."} instead of full stream
	var aux map[string]any
	if err := json.Unmarshal(resp, &aux); err == nil {
		if idRaw, ok := aux["stream_id"]; ok {
			if id, ok := idRaw.(string); ok {
				out.ID = id
				return &out, nil
			}
		}
	}
	// Fallback: try to unmarshal as Stream
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetStream(id string) (*Stream, error) {
	path := fmt.Sprintf("/streams/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/streams/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Stream
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateStream(id string, s *Stream) (*Stream, error) {
	path := fmt.Sprintf("/streams/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/streams/%s", id)
	}
	resp, err := c.doRequest("PUT", path, s)
	if err != nil {
		return nil, err
	}
	var out Stream
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteStream(id string) error {
	path := fmt.Sprintf("/streams/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/streams/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

type Input struct {
	ID            string                 `json:"id,omitempty"`
	Title         string                 `json:"title"`
	Type          string                 `json:"type"`
	Global        bool                   `json:"global,omitempty"`
	Node          string                 `json:"node,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

func (c *Client) CreateInput(in *Input) (*Input, error) {
	path := "/system/inputs"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/inputs"
	}
	resp, err := c.doRequest("POST", path, in)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetInput(id string) (*Input, error) {
	path := fmt.Sprintf("/system/inputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/inputs/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateInput(id string, in *Input) (*Input, error) {
	path := fmt.Sprintf("/system/inputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/inputs/%s", id)
	}
	resp, err := c.doRequest("PUT", path, in)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteInput(id string) error {
	path := fmt.Sprintf("/system/inputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/inputs/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ===== Stream Rules (Streams) =====
// Full-featured stream rules management through dedicated endpoints.

type StreamRule struct {
	ID          string `json:"id,omitempty"`
	Field       string `json:"field"`
	Type        int    `json:"type"`
	Value       string `json:"value"`
	Inverted    bool   `json:"inverted,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListStreamRules returns a list of stream rules for the given stream.
func (c *Client) ListStreamRules(streamID string) ([]StreamRule, error) {
	base := fmt.Sprintf("/streams/%s/rules", streamID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		base = fmt.Sprintf("/api/streams/%s/rules", streamID)
	}
	resp, err := c.doRequest("GET", base, nil)
	if err != nil {
		return nil, err
	}
	// Common Graylog format: {"stream_rules": [ ... ]}
	var wrapper struct {
		StreamRules []StreamRule `json:"stream_rules"`
	}
	if err := json.Unmarshal(resp, &wrapper); err == nil && wrapper.StreamRules != nil {
		return wrapper.StreamRules, nil
	}
	// Be lenient if API returns array directly
	var direct []StreamRule
	if err := json.Unmarshal(resp, &direct); err == nil && direct != nil {
		return direct, nil
	}
	return nil, errors.New("unexpected stream rules response format")
}

// ===== LDAP Settings =====
// Graylog exposes global LDAP settings as a singleton resource.

type LDAPSettings struct {
	Enabled               bool                   `json:"enabled"`
	SystemUsername        string                 `json:"system_username,omitempty"`
	SystemPassword        string                 `json:"system_password,omitempty"`
	LDAPURI               string                 `json:"ldap_uri,omitempty"`
	SearchBase            string                 `json:"search_base,omitempty"`
	SearchPattern         string                 `json:"search_pattern,omitempty"`
	UserUniqueIDAttribute string                 `json:"user_unique_id_attribute,omitempty"`
	GroupSearchBase       string                 `json:"group_search_base,omitempty"`
	GroupSearchPattern    string                 `json:"group_search_pattern,omitempty"`
	DefaultGroup          string                 `json:"default_group,omitempty"`
	UseStartTLS           bool                   `json:"use_start_tls,omitempty"`
	TrustAllCertificates  bool                   `json:"trust_all_certificates,omitempty"`
	ActiveDirectory       bool                   `json:"active_directory,omitempty"`
	DisplayNameAttribute  string                 `json:"display_name_attribute,omitempty"`
	EmailAttribute        string                 `json:"email_attribute,omitempty"`
	AdditionalFields      map[string]interface{} `json:"-"` // pass-through extras
}

// GetLDAPSettings fetches current LDAP settings.
func (c *Client) GetLDAPSettings() (*LDAPSettings, error) {
	path := "/system/ldap/settings"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/ldap/settings"
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out LDAPSettings
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateLDAPSettings updates LDAP settings (singleton upsert).
func (c *Client) UpdateLDAPSettings(s *LDAPSettings) (*LDAPSettings, error) {
	path := "/system/ldap/settings"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/ldap/settings"
	}
	// Marshal as-is, Graylog ignores unknown fields.
	resp, err := c.doRequest("PUT", path, s)
	if err != nil {
		return nil, err
	}
	var out LDAPSettings
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

// ===== Outputs =====

type Output struct {
	ID            string                 `json:"id,omitempty"`
	Title         string                 `json:"title"`
	Type          string                 `json:"type"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

func (c *Client) CreateOutput(o *Output) (*Output, error) {
	path := "/system/outputs"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/outputs"
	}
	resp, err := c.doRequest("POST", path, o)
	if err != nil {
		return nil, err
	}
	var out Output
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetOutput(id string) (*Output, error) {
	path := fmt.Sprintf("/system/outputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/outputs/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Output
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateOutput(id string, o *Output) (*Output, error) {
	path := fmt.Sprintf("/system/outputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/outputs/%s", id)
	}
	resp, err := c.doRequest("PUT", path, o)
	if err != nil {
		return nil, err
	}
	var out Output
	_ = json.Unmarshal(resp, &out)
	if out.ID == "" {
		// Some versions return empty; keep id
		out = *o
		out.ID = id
	}
	return &out, nil
}

func (c *Client) DeleteOutput(id string) error {
	path := fmt.Sprintf("/system/outputs/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/outputs/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) AttachOutputToStream(streamID, outputID string) error {
	// Preferred API: POST /streams/{streamID}/outputs with body {output_id}
	path := fmt.Sprintf("/streams/%s/outputs", streamID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/streams/%s/outputs", streamID)
	}
	body := map[string]string{"output_id": outputID}
	if _, err := c.doRequest("POST", path, body); err == nil {
		return nil
	} else {
		// Fallback to legacy endpoint: POST /streams/{id}/outputs/{outputId}
		legacy := fmt.Sprintf("/streams/%s/outputs/%s", streamID, outputID)
		if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
			legacy = fmt.Sprintf("/api/streams/%s/outputs/%s", streamID, outputID)
		}
		_, err2 := c.doRequest("POST", legacy, nil)
		return err2
	}
}

func (c *Client) DetachOutputFromStream(streamID, outputID string) error {
	// DELETE /streams/{id}/outputs/{output_id}
	path := fmt.Sprintf("/streams/%s/outputs/%s", streamID, outputID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/streams/%s/outputs/%s", streamID, outputID)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ===== Roles =====

type Role struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	ReadOnly    bool     `json:"read_only,omitempty"`
}

func (c *Client) CreateRole(r *Role) (*Role, error) {
	path := "/roles"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/roles"
	}
	resp, err := c.doRequest("POST", path, r)
	if err != nil {
		return nil, err
	}
	var out Role
	_ = json.Unmarshal(resp, &out)
	if out.Name == "" {
		out = *r
	}
	return &out, nil
}

func (c *Client) GetRole(name string) (*Role, error) {
	path := fmt.Sprintf("/roles/%s", name)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/roles/%s", name)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Role
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateRole(name string, r *Role) (*Role, error) {
	path := fmt.Sprintf("/roles/%s", name)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/roles/%s", name)
	}
	resp, err := c.doRequest("PUT", path, r)
	if err != nil {
		return nil, err
	}
	var out Role
	_ = json.Unmarshal(resp, &out)
	if out.Name == "" {
		out = *r
		out.Name = name
	}
	return &out, nil
}

func (c *Client) DeleteRole(name string) error {
	path := fmt.Sprintf("/roles/%s", name)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/roles/%s", name)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// CreateStreamRule creates a rule for the given stream and returns the created rule (with ID, if provided by API).
func (c *Client) CreateStreamRule(streamID string, rule *StreamRule) (*StreamRule, error) {
	base := fmt.Sprintf("/streams/%s/rules", streamID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		base = fmt.Sprintf("/api/streams/%s/rules", streamID)
	}
	resp, err := c.doRequest("POST", base, rule)
	if err != nil {
		return nil, err
	}
	var out StreamRule
	// Some versions return wrapper with id only; try to unmarshal leniently
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

// DeleteStreamRule deletes a specific rule by its ID from the given stream.
func (c *Client) DeleteStreamRule(streamID, ruleID string) error {
	base := fmt.Sprintf("/streams/%s/rules/%s", streamID, ruleID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		base = fmt.Sprintf("/api/streams/%s/rules/%s", streamID, ruleID)
	}
	_, err := c.doRequest("DELETE", base, nil)
	return err
}

// ===== Extractors (Inputs) =====
// We keep extractor payloads as free-form maps to allow full flexibility across Graylog versions.

// ListInputExtractors returns a flat list of extractor objects for the specified input.
func (c *Client) ListInputExtractors(inputID string) ([]map[string]interface{}, error) {
	base := fmt.Sprintf("/system/inputs/%s/extractors", inputID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		base = fmt.Sprintf("/api/system/inputs/%s/extractors", inputID)
	}
	resp, err := c.doRequest("GET", base, nil)
	if err != nil {
		return nil, err
	}
	// Graylog wraps response like {"extractors": [ ... ]}
	var wrapper struct {
		Extractors []map[string]interface{} `json:"extractors"`
	}
	if err := json.Unmarshal(resp, &wrapper); err == nil && wrapper.Extractors != nil {
		return wrapper.Extractors, nil
	}
	// Some versions may return an array directly (be lenient)
	var direct []map[string]interface{}
	if err := json.Unmarshal(resp, &direct); err == nil && direct != nil {
		return direct, nil
	}
	return nil, errors.New("unexpected extractors response format")
}

// CreateInputExtractor creates an extractor for the specified input and returns the created object.
func (c *Client) CreateInputExtractor(inputID string, extractor map[string]interface{}) (map[string]interface{}, error) {
	base := fmt.Sprintf("/system/inputs/%s/extractors", inputID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		base = fmt.Sprintf("/api/system/inputs/%s/extractors", inputID)
	}
	resp, err := c.doRequest("POST", base, extractor)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	_ = json.Unmarshal(resp, &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

// DeleteInputExtractor deletes a specific extractor by id for the given input.
func (c *Client) DeleteInputExtractor(inputID, extractorID string) error {
	path := fmt.Sprintf("/system/inputs/%s/extractors/%s", inputID, extractorID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/inputs/%s/extractors/%s", inputID, extractorID)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

type IndexSet struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	IndexPrefix string `json:"index_prefix"`
	Shards      int    `json:"shards,omitempty"`
	Replicas    int    `json:"replicas,omitempty"`
	// Legacy simple names kept for backward compatibility in provider code.
	// Graylog 5.x+ requires rotation_strategy_class and rotation/retention config objects.
	// Do not marshal/unmarshal these legacy simple fields to avoid conflicts with full config objects.
	RotationStrategy  string `json:"-"`
	RetentionStrategy string `json:"-"`
	// Full strategy support (preferred for Graylog 5.x+)
	RotationStrategyClass    string         `json:"rotation_strategy_class,omitempty"`
	RotationStrategyConfig   map[string]any `json:"rotation_strategy,omitempty"`
	RetentionStrategyClass   string         `json:"retention_strategy_class,omitempty"`
	RetentionStrategyConfig  map[string]any `json:"retention_strategy,omitempty"`
	IndexAnalyzer            string         `json:"index_analyzer,omitempty"`
	FieldTypeRefreshInterval int            `json:"field_type_refresh_interval,omitempty"`
	Default                  bool           `json:"default,omitempty"`
	// Optimization-related settings can be required by some Graylog versions
	IndexOptimizationMaxNumSegments int  `json:"index_optimization_max_num_segments,omitempty"`
	IndexOptimizationDisabled       bool `json:"index_optimization_disabled,omitempty"`
}

// ListIndexSets returns all index sets (used to find the default/writable set).
func (c *Client) ListIndexSets() ([]IndexSet, error) {
	path := "/system/indices/index_sets"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/indices/index_sets"
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Graylog usually wraps list into {"index_sets": [...]} but be liberal in parsing
	var wrapper struct {
		IndexSets []IndexSet `json:"index_sets"`
	}
	if err := json.Unmarshal(resp, &wrapper); err == nil && len(wrapper.IndexSets) > 0 {
		return wrapper.IndexSets, nil
	}
	// Fallback: try to unmarshal as a plain array
	var arr []IndexSet
	if err := json.Unmarshal(resp, &arr); err == nil {
		return arr, nil
	}
	// If neither worked, return empty slice
	return []IndexSet{}, nil
}

func (c *Client) CreateIndexSet(is *IndexSet) (*IndexSet, error) {
	// Some Graylog/OpenSearch setups require a non-null index analyzer. Use a safe default
	// if not provided explicitly to keep compatibility across Graylog v5/v6/v7.
	if is.IndexAnalyzer == "" {
		is.IndexAnalyzer = "standard"
	}
	if is.FieldTypeRefreshInterval == 0 {
		is.FieldTypeRefreshInterval = 5000
	}
	// Ensure valid defaults for optimization settings across versions
	if is.IndexOptimizationMaxNumSegments == 0 {
		is.IndexOptimizationMaxNumSegments = 1
	}
	// Keep optimization enabled by default unless explicitly disabled
	// (do not force true/false if user set it)
	path := "/system/indices/index_sets"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/indices/index_sets"
	}
	// Build a request compatible with Graylog 5.x+ API which expects
	// rotation_strategy_class/retention_strategy_class and corresponding configs.
	type indexSetRequest struct {
		Title                        string         `json:"title"`
		Description                  string         `json:"description,omitempty"`
		IndexPrefix                  string         `json:"index_prefix"`
		Shards                       int            `json:"shards,omitempty"`
		Replicas                     int            `json:"replicas,omitempty"`
		IndexAnalyzer                string         `json:"index_analyzer,omitempty"`
		FieldTypeRefreshInterval     int            `json:"field_type_refresh_interval,omitempty"`
		Default                      bool           `json:"default,omitempty"`
		IndexOptimizationMaxSegments int            `json:"index_optimization_max_num_segments,omitempty"`
		IndexOptimizationDisabled    bool           `json:"index_optimization_disabled,omitempty"`
		CreationDate                 string         `json:"creation_date,omitempty"`
		RotationStrategyClass        string         `json:"rotation_strategy_class,omitempty"`
		RotationStrategyCfg          map[string]any `json:"rotation_strategy,omitempty"`
		RetentionStrategyClass       string         `json:"retention_strategy_class,omitempty"`
		RetentionStrategyCfg         map[string]any `json:"retention_strategy,omitempty"`
	}

	// Choose safe defaults when not explicitly provided by the caller.
	// MessageCountRotationStrategy + DeletionRetentionStrategy are broadly supported.
	rotClass := is.RotationStrategyClass
	rotCfg := is.RotationStrategyConfig
	retClass := is.RetentionStrategyClass
	retCfg := is.RetentionStrategyConfig

	if rotClass == "" {
		rotClass = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
	}
	if rotCfg == nil || len(rotCfg) == 0 {
		rotCfg = map[string]any{
			"type":               "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig",
			"max_docs_per_index": 20000000,
		}
	}
	if retClass == "" {
		retClass = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
	}
	if retCfg == nil || len(retCfg) == 0 {
		retCfg = map[string]any{
			"type":                  "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig",
			"max_number_of_indices": 20,
		}
	}

	// If legacy simple names were provided, try to map them to classes.
	switch strings.ToLower(is.RotationStrategy) {
	case "", "count", "message_count", "messages":
		// keep defaults
	}
	switch strings.ToLower(is.RetentionStrategy) {
	case "", "delete", "deletion":
		// keep defaults
	}

	req := indexSetRequest{
		Title:                        is.Title,
		Description:                  is.Description,
		IndexPrefix:                  is.IndexPrefix,
		Shards:                       is.Shards,
		Replicas:                     is.Replicas,
		IndexAnalyzer:                is.IndexAnalyzer,
		FieldTypeRefreshInterval:     is.FieldTypeRefreshInterval,
		Default:                      is.Default,
		IndexOptimizationMaxSegments: is.IndexOptimizationMaxNumSegments,
		IndexOptimizationDisabled:    is.IndexOptimizationDisabled,
		CreationDate:                 time.Now().UTC().Format(time.RFC3339Nano),
		RotationStrategyClass:        rotClass,
		RotationStrategyCfg:          rotCfg,
		RetentionStrategyClass:       retClass,
		RetentionStrategyCfg:         retCfg,
	}

	resp, err := c.doRequest("POST", path, req)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetIndexSet(id string) (*IndexSet, error) {
	path := fmt.Sprintf("/system/indices/index_sets/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/indices/index_sets/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateIndexSet(id string, is *IndexSet) (*IndexSet, error) {
	path := fmt.Sprintf("/system/indices/index_sets/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/indices/index_sets/%s", id)
	}
	// Mirror CreateIndexSet request shape for updates.
	type indexSetRequest struct {
		Title                        string         `json:"title"`
		Description                  string         `json:"description,omitempty"`
		IndexPrefix                  string         `json:"index_prefix"`
		Shards                       int            `json:"shards,omitempty"`
		Replicas                     int            `json:"replicas,omitempty"`
		IndexAnalyzer                string         `json:"index_analyzer,omitempty"`
		FieldTypeRefreshInterval     int            `json:"field_type_refresh_interval,omitempty"`
		Default                      bool           `json:"default,omitempty"`
		IndexOptimizationMaxSegments int            `json:"index_optimization_max_num_segments,omitempty"`
		IndexOptimizationDisabled    bool           `json:"index_optimization_disabled,omitempty"`
		CreationDate                 string         `json:"creation_date,omitempty"`
		RotationStrategyClass        string         `json:"rotation_strategy_class,omitempty"`
		RotationStrategyCfg          map[string]any `json:"rotation_strategy,omitempty"`
		RetentionStrategyClass       string         `json:"retention_strategy_class,omitempty"`
		RetentionStrategyCfg         map[string]any `json:"retention_strategy,omitempty"`
	}

	rotClass := is.RotationStrategyClass
	rotCfg := is.RotationStrategyConfig
	retClass := is.RetentionStrategyClass
	retCfg := is.RetentionStrategyConfig

	if rotClass == "" {
		rotClass = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
	}
	if rotCfg == nil || len(rotCfg) == 0 {
		rotCfg = map[string]any{
			"type":               "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig",
			"max_docs_per_index": 20000000,
		}
	}
	if retClass == "" {
		retClass = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
	}
	if retCfg == nil || len(retCfg) == 0 {
		retCfg = map[string]any{
			"type":                  "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig",
			"max_number_of_indices": 20,
		}
	}

	switch strings.ToLower(is.RotationStrategy) {
	case "", "count", "message_count", "messages":
	}
	switch strings.ToLower(is.RetentionStrategy) {
	case "", "delete", "deletion":
	}

	req := indexSetRequest{
		Title:                        is.Title,
		Description:                  is.Description,
		IndexPrefix:                  is.IndexPrefix,
		Shards:                       is.Shards,
		Replicas:                     is.Replicas,
		IndexAnalyzer:                is.IndexAnalyzer,
		FieldTypeRefreshInterval:     is.FieldTypeRefreshInterval,
		Default:                      is.Default,
		IndexOptimizationMaxSegments: is.IndexOptimizationMaxNumSegments,
		IndexOptimizationDisabled:    is.IndexOptimizationDisabled,
		CreationDate:                 time.Now().UTC().Format(time.RFC3339Nano),
		RotationStrategyClass:        rotClass,
		RotationStrategyCfg:          rotCfg,
		RetentionStrategyClass:       retClass,
		RetentionStrategyCfg:         retCfg,
	}

	resp, err := c.doRequest("PUT", path, req)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteIndexSet(id string) error {
	path := fmt.Sprintf("/system/indices/index_sets/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/indices/index_sets/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ---- Pipelines ----

type Pipeline struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// In Graylog, pipeline can be represented by source (stages and rules) as a string
	Source string `json:"source,omitempty"`
}

func (c *Client) CreatePipeline(p *Pipeline) (*Pipeline, error) {
	path := "/system/pipelines/pipeline"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/system/pipelines/pipeline"
	}
	resp, err := c.doRequest("POST", path, p)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetPipeline(id string) (*Pipeline, error) {
	path := fmt.Sprintf("/system/pipelines/pipeline/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdatePipeline(id string, p *Pipeline) (*Pipeline, error) {
	path := fmt.Sprintf("/system/pipelines/pipeline/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
	}
	resp, err := c.doRequest("PUT", path, p)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeletePipeline(id string) error {
	path := fmt.Sprintf("/system/pipelines/pipeline/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ---- Dashboards ----

type Dashboard struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func (c *Client) CreateDashboard(d *Dashboard) (*Dashboard, error) {
	path := "/dashboards"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/dashboards"
	}
	resp, err := c.doRequest("POST", path, d)
	if err != nil {
		return nil, err
	}
	var out Dashboard
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetDashboard(id string) (*Dashboard, error) {
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Dashboard
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateDashboard(id string, d *Dashboard) (*Dashboard, error) {
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s", id)
	}
	resp, err := c.doRequest("PUT", path, d)
	if err != nil {
		return nil, err
	}
	var out Dashboard
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteDashboard(id string) error {
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) ListDashboards() ([]Dashboard, error) {
	path := "/dashboards"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/dashboards"
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Graylog may return { dashboards: [...] } or a raw array depending on version
	var arr []Dashboard
	if len(resp) > 0 && resp[0] == '[' {
		_ = json.Unmarshal(resp, &arr)
		return arr, nil
	}
	var wrap struct {
		Dashboards []Dashboard `json:"dashboards"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Dashboards != nil {
		return wrap.Dashboards, nil
	}
	return arr, nil
}

// ---- Dashboard Widgets (classic dashboards) ----

type DashboardWidget struct {
	ID            string                 `json:"id,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Type          string                 `json:"type"`
	CacheTime     int                    `json:"cache_time,omitempty"`
	Configuration map[string]interface{} `json:"config,omitempty"`
}

func (c *Client) CreateDashboardWidget(dashboardID string, w *DashboardWidget) (*DashboardWidget, error) {
	path := fmt.Sprintf("/dashboards/%s/widgets", dashboardID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s/widgets", dashboardID)
	}
	resp, err := c.doRequest("POST", path, w)
	if err != nil {
		return nil, err
	}
	var out DashboardWidget
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetDashboardWidget(dashboardID, widgetID string) (*DashboardWidget, error) {
	path := fmt.Sprintf("/dashboards/%s/widgets/%s", dashboardID, widgetID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s/widgets/%s", dashboardID, widgetID)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out DashboardWidget
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateDashboardWidget(dashboardID, widgetID string, w *DashboardWidget) (*DashboardWidget, error) {
	path := fmt.Sprintf("/dashboards/%s/widgets/%s", dashboardID, widgetID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s/widgets/%s", dashboardID, widgetID)
	}
	resp, err := c.doRequest("PUT", path, w)
	if err != nil {
		return nil, err
	}
	var out DashboardWidget
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteDashboardWidget(dashboardID, widgetID string) error {
	path := fmt.Sprintf("/dashboards/%s/widgets/%s", dashboardID, widgetID)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/dashboards/%s/widgets/%s", dashboardID, widgetID)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ---- Alerts (Event Definitions) ----

type EventDefinition struct {
	ID              string                 `json:"id,omitempty"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description,omitempty"`
	Priority        int                    `json:"priority,omitempty"`
	Alert           bool                   `json:"alert,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	NotificationIDs []string               `json:"notification_ids,omitempty"`
	// Graylog 5 requires additional fields
	KeySpec              []string               `json:"key_spec,omitempty"`
	NotificationSettings map[string]interface{} `json:"notification_settings,omitempty"`
}

func (c *Client) CreateEventDefinition(ed *EventDefinition) (*EventDefinition, error) {
	path := "/events/definitions"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/events/definitions"
	}
	// Ensure required defaults for Graylog 5 compatibility
	if ed.KeySpec == nil {
		ed.KeySpec = []string{}
	}
	if ed.NotificationSettings == nil {
		ed.NotificationSettings = map[string]interface{}{
			"grace_period_ms": 0,
			"backlog_size":    0,
		}
	}
	var body any = ed
	if c.APIVersion == APIV5 {
		// GL5 expects snake_case fields key_spec/notification_settings
		req := map[string]any{
			"title":       ed.Title,
			"description": ed.Description,
			"priority":    ed.Priority,
			"alert":       ed.Alert,
			"config":      ed.Config,
			// GL5 uses "notifications" objects; omit unknown notification_ids
			"notifications":         []any{},
			"notification_settings": ed.NotificationSettings,
			"key_spec":              ed.KeySpec,
		}
		body = req
	}
	resp, err := c.doRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	var out EventDefinition
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetEventDefinition(id string) (*EventDefinition, error) {
	path := fmt.Sprintf("/events/definitions/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/definitions/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out EventDefinition
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateEventDefinition(id string, ed *EventDefinition) (*EventDefinition, error) {
	path := fmt.Sprintf("/events/definitions/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/definitions/%s", id)
	}
	if ed.KeySpec == nil {
		ed.KeySpec = []string{}
	}
	if ed.NotificationSettings == nil {
		ed.NotificationSettings = map[string]interface{}{
			"grace_period_ms": 0,
			"backlog_size":    0,
		}
	}
	var body any = ed
	if c.APIVersion == APIV5 {
		req := map[string]any{
			"title":                 ed.Title,
			"description":           ed.Description,
			"priority":              ed.Priority,
			"alert":                 ed.Alert,
			"config":                ed.Config,
			"notifications":         []any{},
			"notification_settings": ed.NotificationSettings,
			"key_spec":              ed.KeySpec,
		}
		body = req
	}
	resp, err := c.doRequest("PUT", path, body)
	if err != nil {
		return nil, err
	}
	var out EventDefinition
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteEventDefinition(id string) error {
	path := fmt.Sprintf("/events/definitions/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/definitions/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ---- Event Notifications ----

type EventNotification struct {
	ID          string                 `json:"id,omitempty"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config"`
}

func (c *Client) CreateEventNotification(n *EventNotification) (*EventNotification, error) {
	path := "/events/notifications"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/events/notifications"
	}
	resp, err := c.doRequest("POST", path, n)
	if err != nil {
		return nil, err
	}
	var out EventNotification
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetEventNotification(id string) (*EventNotification, error) {
	path := fmt.Sprintf("/events/notifications/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/notifications/%s", id)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out EventNotification
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateEventNotification(id string, n *EventNotification) (*EventNotification, error) {
	path := fmt.Sprintf("/events/notifications/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/notifications/%s", id)
	}
	resp, err := c.doRequest("PUT", path, n)
	if err != nil {
		return nil, err
	}
	var out EventNotification
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteEventNotification(id string) error {
	path := fmt.Sprintf("/events/notifications/%s", id)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/events/notifications/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) ListEventNotifications() ([]EventNotification, error) {
	path := "/events/notifications"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/events/notifications"
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Can be { notifications: [...] } or array
	var arr []EventNotification
	if len(resp) > 0 && resp[0] == '[' {
		_ = json.Unmarshal(resp, &arr)
		return arr, nil
	}
	var wrap struct {
		Notifications []EventNotification `json:"notifications"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Notifications != nil {
		return wrap.Notifications, nil
	}
	return arr, nil
}

// ---- Users ----

type User struct {
	Username         string   `json:"username"`
	FullName         string   `json:"full_name,omitempty"`
	Email            string   `json:"email,omitempty"`
	Roles            []string `json:"roles,omitempty"`
	Timezone         string   `json:"timezone,omitempty"`
	SessionTimeoutMs int64    `json:"session_timeout_ms,omitempty"`
	Disabled         bool     `json:"disabled,omitempty"`
	Password         string   `json:"password,omitempty"`
}

func (c *Client) CreateUser(u *User) (*User, error) {
	path := "/users"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/users"
	}
	// Graylog 5 expects first_name/last_name instead of full_name in CreateUserRequest
	var body any = u
	if c.APIVersion == APIV5 {
		first := u.FullName
		last := ""
		// best-effort split "First Last" -> first_name/last_name
		if idx := strings.Index(first, " "); idx > 0 {
			last = strings.TrimSpace(first[idx+1:])
			first = strings.TrimSpace(first[:idx])
		}
		m := map[string]any{
			"username":           u.Username,
			"first_name":         first,
			"last_name":          last,
			"email":              u.Email,
			"roles":              u.Roles,
			"permissions":        []string{},
			"timezone":           u.Timezone,
			"session_timeout_ms": u.SessionTimeoutMs,
			"password":           u.Password,
		}
		// 'disabled' is not a recognized property in v5 CreateUserRequest; set via update if needed
		body = m
	}
	resp, err := c.doRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	var out User
	_ = json.Unmarshal(resp, &out)
	// Some Graylog versions (e.g., 5.x) may not return the created entity body
	if out.Username == "" {
		return c.GetUser(u.Username)
	}
	return &out, nil
}

func (c *Client) GetUser(username string) (*User, error) {
	path := fmt.Sprintf("/users/%s", username)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/users/%s", username)
	}
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out User
	_ = json.Unmarshal(resp, &out)
	// Не возвращает пароль — и это нормально
	out.Password = ""
	return &out, nil
}

func (c *Client) UpdateUser(username string, u *User) (*User, error) {
	path := fmt.Sprintf("/users/%s", username)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/users/%s", username)
	}
	// Копия без пароля для основного апдейта
	body := *u
	body.Password = ""
	_, err := c.doRequest("PUT", path, &body)
	if err != nil {
		return nil, err
	}
	// Если задан пароль — выполнить отдельный вызов смены пароля
	if u.Password != "" {
		ppath := fmt.Sprintf("/users/%s/password", username)
		if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
			ppath = fmt.Sprintf("/api/users/%s/password", username)
		}
		_, err := c.doRequest("PUT", ppath, map[string]string{"password": u.Password})
		if err != nil {
			return nil, err
		}
	}
	// Вернуть актуальное состояние
	return c.GetUser(username)
}

func (c *Client) DeleteUser(username string) error {
	path := fmt.Sprintf("/users/%s", username)
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = fmt.Sprintf("/api/users/%s", username)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}
