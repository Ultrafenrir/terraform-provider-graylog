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

// ---- Alerts (Event Definitions) ----

type EventDefinition struct {
	ID              string                 `json:"id,omitempty"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description,omitempty"`
	Priority        int                    `json:"priority,omitempty"`
	Alert           bool                   `json:"alert,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	NotificationIDs []string               `json:"notification_ids,omitempty"`
}

func (c *Client) CreateEventDefinition(ed *EventDefinition) (*EventDefinition, error) {
	path := "/events/definitions"
	if c.APIVersion == APIV6 || c.APIVersion == APIV7 {
		path = "/api/events/definitions"
	}
	resp, err := c.doRequest("POST", path, ed)
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
	resp, err := c.doRequest("PUT", path, ed)
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
