package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
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
	// ctx — контекст по умолчанию для всех запросов (может быть переопределён через WithContext)
	ctx    context.Context
	logger Logger

	// Расширенные настройки аутентификации/транспорта (для 0.3.0)
	AuthMethod       string // auto|basic_userpass|basic_token|basic_legacy_b64|bearer|none
	Username         string
	Password         string
	APIToken         string
	APITokenPassword string // по умолчанию пустой; иногда используют "token"
	BearerToken      string

	// TLS/HTTP
	InsecureSkipVerify bool
	CABundlePath       string
	ClientCertPath     string
	ClientKeyPath      string

	// OpenSearch support (auxiliary client sharing the same http.Transport)
	OSBaseURL string

	// Capabilities cache (lazy-probed)
	capabilities *Capabilities
	capOnce      sync.Once
}

func New(baseURL, token string) *Client {
	c := &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
		RetryWait:  time.Second,
		ctx:        context.Background(),
		logger:     NoopLogger{},
	}
	// Normalize base URL: store without /api suffix; add /api prefix in request paths based on API version
	if strings.HasSuffix(c.BaseURL, "/api") {
		c.BaseURL = strings.TrimSuffix(c.BaseURL, "/api")
	}
	c.detectVersion()
	return c
}

// Options — extended client options for Terraform provider v0.3.0
type Options struct {
	// Auth
	AuthMethod       string // auto|basic_userpass|basic_token|basic_legacy_b64|bearer|none
	Username         string
	Password         string
	APIToken         string
	APITokenPassword string
	BearerToken      string
	LegacyTokenB64   string // compatibility with legacy 'token' field

	// Transport/TLS
	InsecureSkipVerify bool
	CABundlePath       string
	ClientCertPath     string
	ClientKeyPath      string

	Timeout    time.Duration
	MaxRetries int
	RetryWait  time.Duration

	// OpenSearch support
	OpenSearchURL string
}

// NewWithOptions creates a client with extended authentication and TLS options
func NewWithOptions(baseURL string, opts Options) *Client {
	c := &Client{
		BaseURL:            strings.TrimRight(baseURL, "/"),
		Token:              opts.LegacyTokenB64,
		HTTP:               buildHTTPClient(opts),
		MaxRetries:         3,
		RetryWait:          time.Second,
		ctx:                context.Background(),
		logger:             NoopLogger{},
		AuthMethod:         opts.AuthMethod,
		Username:           opts.Username,
		Password:           opts.Password,
		APIToken:           opts.APIToken,
		APITokenPassword:   opts.APITokenPassword,
		BearerToken:        opts.BearerToken,
		InsecureSkipVerify: opts.InsecureSkipVerify,
		CABundlePath:       opts.CABundlePath,
		ClientCertPath:     opts.ClientCertPath,
		ClientKeyPath:      opts.ClientKeyPath,
	}
	// OpenSearch base URL (optional)
	if opts.OpenSearchURL != "" {
		c.OSBaseURL = strings.TrimRight(opts.OpenSearchURL, "/")
	}
	if opts.MaxRetries > 0 {
		c.MaxRetries = opts.MaxRetries
	}
	if opts.RetryWait > 0 {
		c.RetryWait = opts.RetryWait
	}
	// Normalize base URL
	if strings.HasSuffix(c.BaseURL, "/api") {
		c.BaseURL = strings.TrimSuffix(c.BaseURL, "/api")
	}
	c.detectVersion()
	return c
}

// Capabilities describes feature availability for current Graylog instance/image
type Capabilities struct {
	ClassicDashboardsCRUD bool
	EventNotifications    bool
	Streams               bool
	IndexSets             bool
}

// GetCapabilities performs a best‑effort probing of supported features and caches the result.
// It uses lightweight list calls where possible and falls back to version heuristics.
func (c *Client) GetCapabilities() *Capabilities {
	c.capOnce.Do(func() {
		caps := &Capabilities{}
		// Streams/Index sets are core in all supported versions
		caps.Streams = true
		caps.IndexSets = true

		// Event Notifications — try to list; if succeeds, feature is present
		if _, err := c.ListEventNotifications(); err == nil {
			caps.EventNotifications = true
		} else {
			caps.EventNotifications = false
		}

		// Classic Dashboards CRUD — generally not on GL 5.x; often missing on 6.x; present on 7.x images supporting legacy dashboards
		if c.APIVersion == APIV7 {
			if _, err := c.ListDashboards(); err == nil {
				caps.ClassicDashboardsCRUD = true
			}
		} else {
			caps.ClassicDashboardsCRUD = false
		}
		c.capabilities = caps
	})
	return c.capabilities
}

func buildHTTPClient(opts Options) *http.Client {
	// Base transport configuration
	tr := &http.Transport{}

	// TLS
	tlsCfg := &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify} //nolint:gosec // user-controlled
	// CA bundle
	if opts.CABundlePath != "" {
		// Attempt to read a custom root CA pool
		if pem, err := os.ReadFile(opts.CABundlePath); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pem) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	// mTLS client cert
	if opts.ClientCertPath != "" && opts.ClientKeyPath != "" {
		if cert, err := tls.LoadX509KeyPair(opts.ClientCertPath, opts.ClientKeyPath); err == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	}
	tr.TLSClientConfig = tlsCfg

	httpClient := &http.Client{Transport: tr}
	if opts.Timeout > 0 {
		httpClient.Timeout = opts.Timeout
	} else {
		httpClient.Timeout = 30 * time.Second
	}
	return httpClient
}

// setAuthHeader sets Authorization header according to the chosen authentication method
func (c *Client) setAuthHeader(req *http.Request) {
	method := c.AuthMethod
	if method == "" || method == "auto" {
		// Auto-select in priority order: user/pass → api_token → bearer → legacy b64
		switch {
		case c.Username != "" || c.Password != "":
			method = "basic_userpass"
		case c.APIToken != "":
			method = "basic_token"
		case c.BearerToken != "":
			method = "bearer"
		case c.Token != "":
			method = "basic_legacy_b64"
		default:
			method = "none"
		}
	}

	switch method {
	case "basic_userpass":
		// user:pass → Base64
		hdr := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password))
		req.Header.Set("Authorization", "Basic "+hdr)
	case "basic_token":
		// token:password → Base64; password is empty by default
		pass := c.APITokenPassword
		hdr := base64.StdEncoding.EncodeToString([]byte(c.APIToken + ":" + pass))
		req.Header.Set("Authorization", "Basic "+hdr)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	case "basic_legacy_b64":
		if c.Token != "" {
			req.Header.Set("Authorization", "Basic "+c.Token)
		}
	case "none":
		// do nothing
	default:
		// unknown method — do not set header
	}
}

func (c *Client) detectVersion() {
	// Assume v5 by default until proven otherwise
	c.APIVersion = APIV5
	headerDetected := false

	// Try to detect version using several base URLs and paths
	bases := []string{c.BaseURL}
	if strings.HasSuffix(c.BaseURL, "/api") {
		bases = append(bases, strings.TrimSuffix(c.BaseURL, "/api"))
	}
	tryPaths := []string{"/api/system", "/system"}
	for _, base := range bases {
		for _, p := range tryPaths {
			req, err := http.NewRequest("GET", base+p, nil)
			if err != nil {
				continue
			}
			// Add same headers as in regular requests so that on 401 server still returns version header
			req.Header.Set("Accept", "application/json")
			req.Header.Set("X-Requested-By", "terraform-provider")
			c.setAuthHeader(req)
			resp, err := c.HTTP.Do(req)
			if err != nil {
				continue
			}
			func() {
				defer resp.Body.Close()
				if resp.StatusCode == 200 || resp.StatusCode == 401 {
					if v := resp.Header.Get("X-Graylog-Version"); v != "" {
						headerDetected = true
						switch {
						case strings.HasPrefix(v, "7."):
							c.APIVersion = APIV7
						case strings.HasPrefix(v, "6."):
							c.APIVersion = APIV6
						default:
							c.APIVersion = APIV5
						}
					}
				}
			}()
			if headerDetected {
				break
			}
		}
		if headerDetected {
			break
		}
	}

	// Если по заголовку определить не удалось — оставляем безопасный дефолт APIV5 без эвристик,
	// чтобы не ошибочно классифицировать 5.x как 7.x.
}

// shouldRetry determines if a request should be retried based on status code
func (c *Client) shouldRetry(statusCode int) bool {
	// Retry on rate limiting and server errors
	return statusCode == 429 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

// WithContext возвращает копию клиента, которая будет использовать переданный контекст
// для всех дальнейших запросов. Это позволяет прокидывать контекст из внешнего кода,
// не меняя сигнатуры всех методов клиента.
func (c *Client) WithContext(ctx context.Context) *Client {
	if ctx == nil {
		ctx = context.Background()
	}
	// Создаём новый клиент без копирования sync.Once (capOnce)
	// Кэш capabilities переносим как есть; capOnce оставляем zero-value.
	return &Client{
		BaseURL:            c.BaseURL,
		Token:              c.Token,
		HTTP:               c.HTTP,
		APIVersion:         c.APIVersion,
		MaxRetries:         c.MaxRetries,
		RetryWait:          c.RetryWait,
		ctx:                ctx,
		logger:             c.logger,
		AuthMethod:         c.AuthMethod,
		Username:           c.Username,
		Password:           c.Password,
		APIToken:           c.APIToken,
		APITokenPassword:   c.APITokenPassword,
		BearerToken:        c.BearerToken,
		InsecureSkipVerify: c.InsecureSkipVerify,
		CABundlePath:       c.CABundlePath,
		ClientCertPath:     c.ClientCertPath,
		ClientKeyPath:      c.ClientKeyPath,
		OSBaseURL:          c.OSBaseURL,
		capabilities:       c.capabilities,
		// capOnce — zero value
	}
}

// SetLogger задаёт реализацию структурированного логирования для клиента.
// По умолчанию используется NoopLogger.
func (c *Client) SetLogger(l Logger) {
	if l == nil {
		c.logger = NoopLogger{}
		return
	}
	c.logger = l
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

		// Используем контекст из клиента (может быть установлен через WithContext)
		ctx := c.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", c.BaseURL, path), buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-By", "terraform-provider")
		c.setAuthHeader(req)

		start := time.Now()
		if os.Getenv("DEBUG_HTTP") == "1" && bodyBytes != nil {
			fmt.Fprintf(os.Stderr, "REQ %s %s: %s\n", method, path, string(bodyBytes))
		}
		c.logger.Debug(ctx, "http_request",
			Fields{
				"method":   method,
				"path":     path,
				"attempt":  attempt + 1,
				"maxRetry": c.MaxRetries + 1,
			},
		)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			// Network error - retry if attempts remain
			lastErr = err
			if attempt < c.MaxRetries {
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
				c.logger.Warn(ctx, "http_request_error",
					Fields{
						"method":  method,
						"path":    path,
						"attempt": attempt + 1,
						"error":   err.Error(),
					},
				)
				time.Sleep(waitTime)
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", c.MaxRetries+1, err)
		}
		defer resp.Body.Close()

		// Opportunistically detect and cache Graylog API version from response headers
		if v := resp.Header.Get("X-Graylog-Version"); v != "" {
			switch {
			case strings.HasPrefix(v, "7."):
				c.APIVersion = APIV7
			case strings.HasPrefix(v, "6."):
				c.APIVersion = APIV6
			default:
				c.APIVersion = APIV5
			}
		}

		// 404 - resource not found, don't retry
		if resp.StatusCode == 404 {
			c.logger.Info(ctx, "http_response",
				Fields{
					"method":   method,
					"path":     path,
					"status":   resp.StatusCode,
					"duration": time.Since(start).String(),
				},
			)
			return nil, ErrNotFound
		}

		// Read response body
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		if os.Getenv("DEBUG_HTTP") == "1" {
			fmt.Fprintf(os.Stderr, "RESP %s %s [%d]: %s\n", method, path, resp.StatusCode, string(b))
		}

		// Check if we should retry
		if resp.StatusCode >= 400 {
			// Пытаемся распарсить структурированную ошибку Graylog
			gerr := ParseGraylogError(resp.StatusCode, b)

			if c.shouldRetry(resp.StatusCode) && attempt < c.MaxRetries {
				lastErr = gerr
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
				c.logger.Warn(ctx, "http_response_retry",
					Fields{
						"method":   method,
						"path":     path,
						"status":   resp.StatusCode,
						"duration": time.Since(start).String(),
						"attempt":  attempt + 1,
						"error":    gerr.Error(),
					},
				)
				time.Sleep(waitTime)
				continue
			}
			c.logger.Error(ctx, "http_response_error",
				Fields{
					"method":   method,
					"path":     path,
					"status":   resp.StatusCode,
					"duration": time.Since(start).String(),
					"error":    gerr.Error(),
				},
			)
			return nil, gerr
		}

		// Success
		c.logger.Debug(ctx, "http_response",
			Fields{
				"method":   method,
				"path":     path,
				"status":   resp.StatusCode,
				"duration": time.Since(start).String(),
			},
		)
		return b, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.MaxRetries+1, lastErr)
}

// osDoRequest performs an HTTP request against OpenSearch base URL (OSBaseURL).
// It shares the same HTTP client/transport and retry policy. No Graylog auth headers are applied.
func (c *Client) osDoRequest(method, path string, body any) ([]byte, error) {
	if c.OSBaseURL == "" {
		return nil, fmt.Errorf("opensearch base URL is not configured")
	}
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		var buf io.Reader
		if bodyBytes != nil {
			buf = bytes.NewBuffer(bodyBytes)
		}
		ctx := c.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", c.OSBaseURL, path), buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		start := time.Now()
		c.logger.Debug(ctx, "os_http_request", Fields{"method": method, "path": path, "attempt": attempt + 1, "maxRetry": c.MaxRetries + 1})
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.MaxRetries {
				waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
				c.logger.Warn(ctx, "os_http_request_error", Fields{"error": err.Error(), "wait": waitTime.String()})
				time.Sleep(waitTime)
				continue
			}
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		c.logger.Debug(ctx, "os_http_response", Fields{"status": resp.StatusCode, "duration_ms": time.Since(start).Milliseconds()})

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return b, nil
		}
		// Map 404 to ErrNotFound for convenience in higher layers
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		// retry on server errors
		if c.shouldRetry(resp.StatusCode) && attempt < c.MaxRetries {
			waitTime := time.Duration(math.Pow(2, float64(attempt))) * c.RetryWait
			c.logger.Warn(ctx, "os_http_retry", Fields{"status": resp.StatusCode, "wait": waitTime.String()})
			time.Sleep(waitTime)
			continue
		}
		// Return a generic error with body for troubleshooting
		return nil, fmt.Errorf("opensearch %s %s failed: status=%d body=%s", method, path, resp.StatusCode, string(b))
	}
	return nil, lastErr
}

// OpenSearch Snapshot Repository helpers
func (c *Client) OSUpsertSnapshotRepository(name, repoType string, settings map[string]any) error {
	body := map[string]any{
		"type":     repoType,
		"settings": settings,
	}
	_, err := c.osDoRequest(http.MethodPut, fmt.Sprintf("/_snapshot/%s", name), body)
	return err
}

func (c *Client) OSGetSnapshotRepository(name string) (string, map[string]any, error) {
	b, err := c.osDoRequest(http.MethodGet, fmt.Sprintf("/_snapshot/%s", name), nil)
	if err != nil {
		return "", nil, err
	}
	// Expected structure: { "<name>": { "type": "fs", "settings": { ... } } }
	var m map[string]struct {
		Type     string         `json:"type"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(b, &m); err == nil {
		if v, ok := m[name]; ok {
			if v.Settings == nil {
				v.Settings = map[string]any{}
			}
			return v.Type, v.Settings, nil
		}
		// If parsed as map but key not present, try array fallback below
	}
	// Some versions may return an array; attempt to parse fallback even if map parsing failed
	var arr []map[string]any
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
		// try to extract type/settings from the first element
		if t, ok := arr[0]["type"].(string); ok {
			if s, ok2 := arr[0]["settings"].(map[string]any); ok2 {
				return t, s, nil
			}
			return t, map[string]any{}, nil
		}
	}
	return "", nil, ErrNotFound
}

func (c *Client) OSDeleteSnapshotRepository(name string) error {
	_, err := c.osDoRequest(http.MethodDelete, fmt.Sprintf("/_snapshot/%s", name), nil)
	return err
}

// GraylogError описывает структурированную ошибку, возвращаемую Graylog API.
type GraylogError struct {
	Status  int                 `json:"-"`
	Type    string              `json:"type,omitempty"`
	Message string              `json:"message,omitempty"`
	Err     string              `json:"error,omitempty"`
	Errors  map[string][]string `json:"errors,omitempty"`
	Details map[string]any      `json:"details,omitempty"`
	Raw     string              `json:"-"`
}

func (e *GraylogError) Error() string {
	// Составляем читаемое сообщение с приоритетом полей
	msg := e.Message
	if msg == "" {
		msg = e.Err
	}
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if len(e.Errors) > 0 {
		// Добавим краткое представление валидационных ошибок
		var b strings.Builder
		b.WriteString(msg)
		b.WriteString(" (validation: ")
		first := true
		for k, arr := range e.Errors {
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(k)
			if len(arr) > 0 {
				b.WriteString("=")
				b.WriteString(arr[0])
			}
		}
		b.WriteString(")")
		msg = b.String()
	}
	return fmt.Sprintf("Graylog API error (status %d): %s", e.Status, msg)
}

// ParseGraylogError пытается разобрать типичные ответы ошибок Graylog во всех поддерживаемых версиях.
func ParseGraylogError(status int, body []byte) error {
	ge := &GraylogError{Status: status, Raw: string(body)}

	// Попытка распарсить JSON
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		// Если это не JSON — вернём обёртку с сырым текстом
		if len(body) > 0 {
			ge.Message = string(body)
		}
		return ge
	}

	// Общие поля
	if v, ok := m["type"].(string); ok {
		ge.Type = v
	}
	if v, ok := m["message"].(string); ok {
		ge.Message = v
	}
	if v, ok := m["error"].(string); ok {
		ge.Err = v
	}

	// Валидационные ошибки могут приходить в разных формах
	ge.Errors = map[string][]string{}
	if v, ok := m["errors"]; ok {
		switch t := v.(type) {
		case map[string]any:
			for field, raw := range t {
				switch rr := raw.(type) {
				case []any:
					for _, it := range rr {
						if s, ok := it.(string); ok {
							ge.Errors[field] = append(ge.Errors[field], s)
						}
					}
				case string:
					ge.Errors[field] = append(ge.Errors[field], rr)
				}
			}
		case []any:
			// иногда приходит массив строк
			var items []string
			for _, it := range t {
				if s, ok := it.(string); ok {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				ge.Errors["_"] = items
			}
		}
	}

	// Сохраним все остальные детали
	ge.Details = map[string]any{}
	for k, v := range m {
		if k == "type" || k == "message" || k == "error" || k == "errors" {
			continue
		}
		ge.Details[k] = v
	}

	return ge
}

// Простое структурированное логирование
type Fields map[string]any

type Logger interface {
	Debug(ctx context.Context, msg string, fields Fields)
	Info(ctx context.Context, msg string, fields Fields)
	Warn(ctx context.Context, msg string, fields Fields)
	Error(ctx context.Context, msg string, fields Fields)
}

// NoopLogger — реализация по умолчанию, ничего не делает
type NoopLogger struct{}

func (NoopLogger) Debug(context.Context, string, Fields) {}
func (NoopLogger) Info(context.Context, string, Fields)  {}
func (NoopLogger) Warn(context.Context, string, Fields)  {}
func (NoopLogger) Error(context.Context, string, Fields) {}

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
	// When true, messages matching this stream are removed from the default stream
	RemoveMatchesFromDefaultStream bool   `json:"remove_matches_from_default_stream,omitempty"`
	Rules                          []Rule `json:"rules,omitempty"`
}

func (c *Client) CreateStream(s *Stream) (*Stream, error) {
	// Unified path for all supported versions
	path := "/api/streams"
	// Default matching type
	matchingType := s.MatchingType
	if matchingType == "" {
		matchingType = "AND"
	}

	// Prepare both request body variants
	// v7+ (CreateEntityRequest with an entity wrapper)
	v7Body := map[string]any{
		"entity": map[string]any{
			"title":         s.Title,
			"description":   s.Description,
			"index_set_id":  s.IndexSetID,
			"matching_type": matchingType,
			// best-effort: some versions may ignore this in create entity wrapper, but include it when supported
			"remove_matches_from_default_stream": s.RemoveMatchesFromDefaultStream,
		},
	}
	// v5/v6 (direct snake_case form, without disabled)
	var rules any = s.Rules
	if rules == nil {
		rules = []any{}
	}
	legacyBody := map[string]any{
		"title":                              s.Title,
		"description":                        s.Description,
		"index_set_id":                       s.IndexSetID,
		"matching_type":                      matchingType,
		"remove_matches_from_default_stream": s.RemoveMatchesFromDefaultStream,
		"rules":                              rules,
	}

	// Strategy: try v7-compatible payload first, then fallback to legacy
	tryBodies := []map[string]any{v7Body, legacyBody}
	// If client is known to be v5/v6 — try legacy first
	if c.APIVersion == APIV5 || c.APIVersion == APIV6 {
		tryBodies = []map[string]any{legacyBody, v7Body}
	}

	var lastResp []byte
	for i, body := range tryBodies {
		resp, err := c.doRequest("POST", path, body)
		if err != nil {
			// On explicit 4xx error try the next payload variant
			lastResp = []byte(err.Error())
			if i+1 < len(tryBodies) {
				continue
			}
			return nil, err
		}
		// Success: try to extract stream_id from different response shapes
		var out Stream
		var aux map[string]any
		if json.Unmarshal(resp, &aux) == nil {
			if idRaw, ok := aux["stream_id"]; ok {
				if id, ok := idRaw.(string); ok {
					out.ID = id
				}
			}
			// sometimes response can be {"stream": {"id": "..."}}
			if stream, ok := aux["stream"].(map[string]any); ok {
				if id, ok := stream["id"].(string); ok && id != "" {
					out.ID = id
				}
			}
		}
		// Fallback: unmarshal directly into Stream
		_ = json.Unmarshal(resp, &out)
		if out.ID != "" {
			// Handle disabled state after creation
			streamPath := fmt.Sprintf("/api/streams/%s", out.ID)

			// Newly created streams default to disabled=true
			// If we want disabled=false, we need to resume it
			if !s.Disabled {
				// Try resume endpoint first (v7)
				_, err := c.doRequest("POST", fmt.Sprintf("%s/resume", streamPath), nil)
				if err != nil {
					// If resume fails, try PUT (v5/v6)
					updateBody := map[string]any{
						"title":                              s.Title,
						"description":                        s.Description,
						"index_set_id":                       s.IndexSetID,
						"matching_type":                      matchingType,
						"remove_matches_from_default_stream": s.RemoveMatchesFromDefaultStream,
						"disabled":                           false,
					}
					c.doRequest("PUT", streamPath, updateBody)
				}
			}

			// Re-read to get actual state
			if got, err := c.GetStream(out.ID); err == nil {
				return got, nil
			}
			return &out, nil
		}
		// If we got here — save the body and continue (in case of the second attempt)
		lastResp = resp
	}
	return nil, fmt.Errorf("failed to create stream: unexpected response %s", string(lastResp))
}

func (c *Client) GetStream(id string) (*Stream, error) {
	// Unified path for all supported versions
	path := fmt.Sprintf("/api/streams/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Stream
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateStream(id string, s *Stream) (*Stream, error) {
	// Graylog API для streams поддерживает только PUT метод на /api/streams/{id}
	path := fmt.Sprintf("/api/streams/%s", id)

	matchingType := s.MatchingType
	if matchingType == "" {
		matchingType = "AND"
	}

	var body any = s
	if c.APIVersion == APIV7 {
		// v7 использует snake_case и не имеет поля 'disabled'
		body = map[string]any{
			"title":                              s.Title,
			"description":                        s.Description,
			"index_set_id":                       s.IndexSetID,
			"matching_type":                      matchingType,
			"remove_matches_from_default_stream": s.RemoveMatchesFromDefaultStream,
		}
	}

	resp, err := c.doRequest("PUT", path, body)
	if err != nil {
		return nil, err
	}

	var out Stream
	_ = json.Unmarshal(resp, &out)

	// Handle disabled state for all versions
	// v7 requires separate pause/resume endpoints
	// v5/v6 may also need them if PUT doesn't update disabled
	if s.Disabled {
		c.doRequest("POST", fmt.Sprintf("%s/pause", path), nil)
	} else {
		c.doRequest("POST", fmt.Sprintf("%s/resume", path), nil)
	}

	// Re-read to get actual state
	got, gerr := c.GetStream(id)
	if gerr == nil {
		return got, nil
	}
	return &out, nil
}

func (c *Client) DeleteStream(id string) error {
	// Unified path for all supported versions
	path := fmt.Sprintf("/api/streams/%s", id)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ListStreams returns all streams (best-effort across versions).
func (c *Client) ListStreams() ([]Stream, error) {
	// Unified path works for 5/6/7 (streams endpoint did not move to /views)
	path := "/api/streams"
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Try common Graylog formats
	// 1) Wrapper { streams: [...] }
	var wrap struct {
		Streams []Stream `json:"streams"`
		Total   int      `json:"total"`
		Page    int      `json:"page"`
		PerPage int      `json:"per_page"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Streams != nil {
		return wrap.Streams, nil
	}
	// 2) Direct array
	var arr []Stream
	if err := json.Unmarshal(resp, &arr); err == nil && arr != nil {
		return arr, nil
	}
	// 3) Some versions may return { data: [...] }
	var alt struct {
		Data     []Stream `json:"data"`
		Elements []Stream `json:"elements"`
		Items    []Stream `json:"items"`
	}
	if err := json.Unmarshal(resp, &alt); err == nil {
		if alt.Data != nil {
			return alt.Data, nil
		}
		if alt.Elements != nil {
			return alt.Elements, nil
		}
		if alt.Items != nil {
			return alt.Items, nil
		}
	}
	// 4) Generic fallback: try to extract "streams" from arbitrary object
	var aux map[string]any
	if err := json.Unmarshal(resp, &aux); err == nil && aux != nil {
		if v, ok := aux["streams"]; ok {
			// normalize to slice of objects
			switch t := v.(type) {
			case []any:
				out := make([]Stream, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var s Stream
					if err := json.Unmarshal(b, &s); err == nil {
						out = append(out, s)
					}
				}
				return out, nil
			case map[string]any:
				out := make([]Stream, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var s Stream
					if err := json.Unmarshal(b, &s); err == nil {
						out = append(out, s)
					}
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("unexpected streams response format")
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
	// Унифицированный путь для всех версий
	path := "/api/system/inputs"
	resp, err := c.doRequest("POST", path, in)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetInput(id string) (*Input, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/inputs/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateInput(id string, in *Input) (*Input, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/inputs/%s", id)
	resp, err := c.doRequest("PUT", path, in)
	if err != nil {
		return nil, err
	}
	var out Input
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteInput(id string) error {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/inputs/%s", id)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ListInputs returns all inputs. Graylog may return either a wrapped object
// like {"inputs": [...]} or a raw array; support both.
func (c *Client) ListInputs() ([]Input, error) {
	path := "/api/system/inputs"
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Try wrapped form first
	var wrap struct {
		Inputs []Input `json:"inputs"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Inputs != nil {
		return wrap.Inputs, nil
	}
	// Fallback to array
	var arr []Input
	if err := json.Unmarshal(resp, &arr); err == nil && arr != nil {
		return arr, nil
	}
	// Be lenient with map[string]any of inputs keyed by id
	var anyMap map[string]any
	if err := json.Unmarshal(resp, &anyMap); err == nil {
		if v, ok := anyMap["inputs"]; ok {
			switch t := v.(type) {
			case []any:
				out := make([]Input, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var s Input
					if err := json.Unmarshal(b, &s); err == nil {
						out = append(out, s)
					}
				}
				return out, nil
			case map[string]any:
				out := make([]Input, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var s Input
					if err := json.Unmarshal(b, &s); err == nil {
						out = append(out, s)
					}
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("unexpected inputs response format")
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
	// Унифицированный путь для всех версий
	base := fmt.Sprintf("/api/streams/%s/rules", streamID)
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
	// Унифицированный путь для всех версий
	path := "/api/system/outputs"
	resp, err := c.doRequest("POST", path, o)
	if err != nil {
		return nil, err
	}
	var out Output
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetOutput(id string) (*Output, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/outputs/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Output
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateOutput(id string, o *Output) (*Output, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/outputs/%s", id)
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
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/outputs/%s", id)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) AttachOutputToStream(streamID, outputID string) error {
	// Try multiple known variants across versions/images, from most specific to generic
	// 1) Legacy style: POST /api/streams/{id}/outputs/{outputId}
	legacy := fmt.Sprintf("/api/streams/%s/outputs/%s", streamID, outputID)
	if _, err := c.doRequest("POST", legacy, nil); err == nil {
		return nil
	}
	// 1b) Some images may expect PUT for legacy-style attach
	if _, err := c.doRequest("PUT", legacy, nil); err == nil {
		return nil
	}
	// 2) Newer style: POST /api/streams/{id}/outputs with JSON body {"output_id":"..."}
	path := fmt.Sprintf("/api/streams/%s/outputs", streamID)
	body := map[string]string{"output_id": outputID}
	if _, err := c.doRequest("POST", path, body); err == nil {
		return nil
	}
	// 2b) Try PUT with body as a last resort
	if _, err := c.doRequest("PUT", path, body); err == nil {
		return nil
	}
	// Return last error (from PUT with body) for context
	_, err := c.doRequest("PUT", legacy, nil)
	return err
}

func (c *Client) DetachOutputFromStream(streamID, outputID string) error {
	// DELETE /api/streams/{id}/outputs/{output_id}
	path := fmt.Sprintf("/api/streams/%s/outputs/%s", streamID, outputID)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ListStreamOutputs returns outputs attached to the given stream.
func (c *Client) ListStreamOutputs(streamID string) ([]Output, error) {
	path := fmt.Sprintf("/api/streams/%s/outputs", streamID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Common formats across versions/images
	// 1) { outputs: [...] }
	var wrap struct {
		Outputs []Output `json:"outputs"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Outputs != nil {
		return wrap.Outputs, nil
	}
	// 2) direct array of outputs
	var arr []Output
	if err := json.Unmarshal(resp, &arr); err == nil && arr != nil {
		return arr, nil
	}
	// 3) { data: [...] } or { elements: [...] } or { items: [...] }
	var alt struct {
		Data     []Output `json:"data"`
		Elements []Output `json:"elements"`
		Items    []Output `json:"items"`
	}
	if err := json.Unmarshal(resp, &alt); err == nil {
		if alt.Data != nil {
			return alt.Data, nil
		}
		if alt.Elements != nil {
			return alt.Elements, nil
		}
		if alt.Items != nil {
			return alt.Items, nil
		}
	}
	// 4) Fallback: try to extract "outputs" key generically
	var aux map[string]any
	if err := json.Unmarshal(resp, &aux); err == nil && aux != nil {
		if v, ok := aux["outputs"]; ok {
			switch t := v.(type) {
			case []any:
				out := make([]Output, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var o Output
					if err := json.Unmarshal(b, &o); err == nil {
						out = append(out, o)
					}
				}
				return out, nil
			case map[string]any:
				out := make([]Output, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var o Output
					if err := json.Unmarshal(b, &o); err == nil {
						out = append(out, o)
					}
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("unexpected stream outputs response format")
}

// ===== Roles =====

type Role struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	ReadOnly    bool     `json:"read_only,omitempty"`
}

func (c *Client) CreateRole(r *Role) (*Role, error) {
	// Унифицированный путь для всех версий
	path := "/api/roles"
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
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/roles/%s", name)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Role
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateRole(name string, r *Role) (*Role, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/roles/%s", name)
	// В разных основных версиях Graylog поведение PUT /roles/{name} отличается:
	// - В 7.x поле name в теле должно отсутствовать (immutable)
	// - В 6.x сервер может требовать присутствие поля name в теле
	// Поэтому сначала отправляем «минимальный» payload без name. В случае 400,
	// явно связанного с отсутствием поля name, повторим запрос с включённым name.

	type updatePayload struct {
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Permissions []string `json:"permissions,omitempty"`
		ReadOnly    bool     `json:"read_only,omitempty"`
	}

	minimal := updatePayload{
		Description: r.Description,
		Permissions: r.Permissions,
		ReadOnly:    r.ReadOnly,
	}

	resp, err := c.doRequest("PUT", path, &minimal)
	if err != nil {
		// Если это структурированная ошибка Graylog и она указывает на проблему
		// c полем name (часто встречается на GL 6.x), попробуем повторить с name.
		if ge, ok := err.(*GraylogError); ok && ge.Status == 400 {
			// Проверим текст ошибки и «сырой» ответ на упоминание name
			raw := strings.ToLower(strings.TrimSpace(ge.Raw))
			msg := strings.ToLower(strings.TrimSpace(ge.Message))
			combined := raw + " " + msg
			if strings.Contains(combined, "name") {
				withName := updatePayload{
					Name:        name,
					Description: r.Description,
					Permissions: r.Permissions,
					ReadOnly:    r.ReadOnly,
				}
				if r2, err2 := c.doRequest("PUT", path, &withName); err2 == nil {
					resp = r2
				} else {
					return nil, err2
				}
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
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
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/roles/%s", name)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// CreateStreamRule creates a rule for the given stream and returns the created rule (with ID, if provided by API).
func (c *Client) CreateStreamRule(streamID string, rule *StreamRule) (*StreamRule, error) {
	// Унифицированный путь для всех версий
	base := fmt.Sprintf("/api/streams/%s/rules", streamID)
	resp, err := c.doRequest("POST", base, rule)
	if err != nil {
		return nil, err
	}
	var out StreamRule
	// Сначала пробуем прямое распаковывание
	if err := json.Unmarshal(resp, &out); err == nil && out.ID != "" {
		return &out, nil
	}
	// Затем пробуем распространённые варианты обёрток
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err == nil {
		// прямой id
		if id, ok := m["id"].(string); ok && id != "" {
			out = *rule
			out.ID = id
			return &out, nil
		}
		// stream_rule_id или rule_id
		if id, ok := m["stream_rule_id"].(string); ok && id != "" {
			out = *rule
			out.ID = id
			return &out, nil
		}
		if id, ok := m["rule_id"].(string); ok && id != "" {
			out = *rule
			out.ID = id
			return &out, nil
		}
		// вложенный объект
		if sr, ok := m["stream_rule"].(map[string]any); ok {
			if id, ok := sr["id"].(string); ok && id != "" {
				out = *rule
				out.ID = id
				return &out, nil
			}
		}
	}
	// Если ничего не подошло — попробуем найти созданное правило через ListStreamRules
	if rules, lerr := c.ListStreamRules(streamID); lerr == nil {
		for _, r := range rules {
			if r.Field == rule.Field && r.Value == rule.Value && r.Type == rule.Type && r.Inverted == rule.Inverted {
				return &r, nil
			}
		}
	}
	// В самом крайнем случае вернём правило без ID
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

// DeleteStreamRule deletes a specific rule by its ID from the given stream.
func (c *Client) DeleteStreamRule(streamID, ruleID string) error {
	// Унифицированный путь для всех версий
	base := fmt.Sprintf("/api/streams/%s/rules/%s", streamID, ruleID)
	_, err := c.doRequest("DELETE", base, nil)
	return err
}

// ===== Extractors (Inputs) =====
// We keep extractor payloads as free-form maps to allow full flexibility across Graylog versions.

// ListInputExtractors returns a flat list of extractor objects for the specified input.
func (c *Client) ListInputExtractors(inputID string) ([]map[string]interface{}, error) {
	// Унифицированный путь для всех версий
	base := fmt.Sprintf("/api/system/inputs/%s/extractors", inputID)
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
	Shards      int    `json:"shards"`
	Replicas    int    `json:"replicas"`
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
	// Optimization-related settings required by Graylog 7.x
	IndexOptimizationMaxNumSegments int  `json:"index_optimization_max_num_segments,omitempty"`
	IndexOptimizationDisabled       bool `json:"index_optimization_disabled"` // Required by Graylog 7.x, removed omitempty
	// Additional fields required by Graylog API
	Writable          bool   `json:"writable,omitempty"`
	CreationDate      string `json:"creation_date,omitempty"`
	CanBeDefault      bool   `json:"can_be_default,omitempty"`
	IndexTemplateType string `json:"index_template_type,omitempty"`
	// Keep IsWritable for backward compatibility but don't serialize it
	IsWritable bool `json:"-"`
}

// ListIndexSets returns all index sets (used to find the default/writable set).
func (c *Client) ListIndexSets() ([]IndexSet, error) {
	// Начиная с Graylog 5 API стабильно доступен под /api/system/indices/index_sets
	path := "/api/system/indices/index_sets"
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
	// Начиная с Graylog 5 API стабильно доступен под /api/system/indices/index_sets
	path := "/api/system/indices/index_sets"
	// Request shapes differ slightly on v7 (camelCase and extra required flags)
	// v5/v6 request (snake_case)
	type indexSetRequestLegacy struct {
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
	// v7 request (camelCase + isWritable required; replicas and indexOptimizationDisabled required)
	type indexSetRequestV7 struct {
		Title                        string         `json:"title"`
		Description                  string         `json:"description,omitempty"`
		IndexPrefix                  string         `json:"indexPrefix"`
		Shards                       int            `json:"shards"`
		Replicas                     int            `json:"replicas"`
		IndexAnalyzer                string         `json:"indexAnalyzer,omitempty"`
		FieldTypeRefreshInterval     int            `json:"fieldTypeRefreshInterval,omitempty"`
		Default                      bool           `json:"default,omitempty"`
		IndexOptimizationMaxSegments int            `json:"indexOptimizationMaxNumSegments,omitempty"`
		IndexOptimizationDisabled    bool           `json:"indexOptimizationDisabled"`
		IsWritable                   bool           `json:"isWritable"`
		CreationDate                 string         `json:"creationDate,omitempty"`
		RotationStrategyClass        string         `json:"rotationStrategyClass"`
		RotationStrategyCfg          map[string]any `json:"rotationStrategy"`
		RetentionStrategyClass       string         `json:"retentionStrategyClass"`
		RetentionStrategyCfg         map[string]any `json:"retentionStrategy"`
		// Duplicate snake_case keys for backward/variant compatibility (Graylog 7 validation can be strict)
		IndexPrefixLegacy                  string `json:"index_prefix,omitempty"`
		IndexOptimizationMaxSegmentsLegacy int    `json:"index_optimization_max_num_segments,omitempty"`
		IndexOptimizationDisabledLegacy    bool   `json:"index_optimization_disabled,omitempty"`
		IsWritableLegacy                   bool   `json:"is_writable,omitempty"`
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
	// If user supplied a config but forgot required discriminator 'type' — infer it from class
	if rotCfg != nil {
		if _, ok := rotCfg["type"]; !ok && rotClass != "" {
			// Most Graylog strategies follow the convention: <...>Strategy -> <...>StrategyConfig
			t := rotClass
			if strings.HasSuffix(t, "Strategy") {
				t = t + "Config"
			}
			rotCfg["type"] = t
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
	// If user supplied a retention config without 'type' — infer it from class
	if retCfg != nil {
		if _, ok := retCfg["type"]; !ok && retClass != "" {
			t := retClass
			if strings.HasSuffix(t, "Strategy") {
				t = t + "Config"
			}
			retCfg["type"] = t
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

	var body any
	if c.APIVersion == APIV7 {
		// На практике для Graylog 7.0.x эндпоинт index_sets ожидает snake_case поля (см. сообщение об ошибке creationDate)
		replicas := is.Replicas
		// По умолчанию оптимизация НЕ отключена (disabled=false)
		idxOptDisabled := is.IndexOptimizationDisabled
		// По умолчанию разрешаем запись
		isWritable := is.IsWritable
		if !isWritable {
			isWritable = true
		}
		body = map[string]any{
			"title":                               is.Title,
			"description":                         is.Description,
			"index_prefix":                        is.IndexPrefix,
			"shards":                              max(1, is.Shards),
			"replicas":                            replicas,
			"index_analyzer":                      is.IndexAnalyzer,
			"field_type_refresh_interval":         is.FieldTypeRefreshInterval,
			"index_optimization_max_num_segments": is.IndexOptimizationMaxNumSegments,
			"index_optimization_disabled":         idxOptDisabled,
			"writable":                            isWritable,
			"creation_date":                       time.Now().UTC().Format(time.RFC3339Nano),
			"rotation_strategy_class":             rotClass,
			"rotation_strategy":                   rotCfg,
			"retention_strategy_class":            retClass,
			"retention_strategy":                  retCfg,
		}
	} else {
		// Формируем payload только со snake_case ключами (legacy),
		// но включаем обязательные поля replicas/index_optimization_disabled/is_writable.
		replicas := is.Replicas
		// По умолчанию оптимизация НЕ отключена (disabled=false), но поле должно присутствовать
		idxOptDisabled := is.IndexOptimizationDisabled
		// По умолчанию разрешаем запись
		isWritable := is.IsWritable
		if !isWritable {
			isWritable = true
		}
		body = map[string]any{
			"title":                               is.Title,
			"description":                         is.Description,
			"index_prefix":                        is.IndexPrefix,
			"shards":                              max(1, is.Shards),
			"replicas":                            replicas,
			"index_analyzer":                      is.IndexAnalyzer,
			"field_type_refresh_interval":         is.FieldTypeRefreshInterval,
			"index_optimization_max_num_segments": is.IndexOptimizationMaxNumSegments,
			"index_optimization_disabled":         idxOptDisabled,
			"writable":                            isWritable,
			"creation_date":                       time.Now().UTC().Format(time.RFC3339Nano),
			"rotation_strategy_class":             rotClass,
			"rotation_strategy":                   rotCfg,
			"retention_strategy_class":            retClass,
			"retention_strategy":                  retCfg,
		}
	}

	resp, err := c.doRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetIndexSet(id string) (*IndexSet, error) {
	// Начиная с Graylog 5 API стабильно доступен под /api/system/indices/index_sets/{id}
	path := fmt.Sprintf("/api/system/indices/index_sets/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateIndexSet(id string, is *IndexSet) (*IndexSet, error) {
	// Graylog API для index sets требует PUT с полным телом объекта
	// Сначала получаем текущее состояние, затем обновляем нужные поля
	path := fmt.Sprintf("/api/system/indices/index_sets/%s", id)

	// Получаем текущий объект из API
	current, err := c.GetIndexSet(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get current index set before update: %w", err)
	}

	// Обновляем только изменяемые поля из запроса
	// Базовые поля, которые всегда обновляем
	if is.Title != "" {
		current.Title = is.Title
	}
	if is.Description != "" {
		current.Description = is.Description
	}
	// Shards and Replicas - always update
	current.Shards = is.Shards
	current.Replicas = is.Replicas

	// Обновляем стратегии если они заданы
	if is.RotationStrategyClass != "" {
		current.RotationStrategyClass = is.RotationStrategyClass
	}
	if is.RotationStrategyConfig != nil && len(is.RotationStrategyConfig) > 0 {
		current.RotationStrategyConfig = is.RotationStrategyConfig
	}
	if is.RetentionStrategyClass != "" {
		current.RetentionStrategyClass = is.RetentionStrategyClass
	}
	if is.RetentionStrategyConfig != nil && len(is.RetentionStrategyConfig) > 0 {
		current.RetentionStrategyConfig = is.RetentionStrategyConfig
	}

	// Update all configurable fields
	current.IndexAnalyzer = is.IndexAnalyzer
	current.FieldTypeRefreshInterval = is.FieldTypeRefreshInterval
	current.IndexOptimizationMaxNumSegments = is.IndexOptimizationMaxNumSegments
	current.IndexOptimizationDisabled = is.IndexOptimizationDisabled
	current.Default = is.Default

	// Установка дефолтных значений для стратегий если они пустые
	if current.RotationStrategyClass == "" {
		current.RotationStrategyClass = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
	}
	if current.RotationStrategyConfig == nil || len(current.RotationStrategyConfig) == 0 {
		current.RotationStrategyConfig = map[string]any{
			"type":               "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategyConfig",
			"max_docs_per_index": 20000000,
		}
	}
	// Добавляем тип конфига если его нет
	if current.RotationStrategyConfig != nil {
		if _, ok := current.RotationStrategyConfig["type"]; !ok && current.RotationStrategyClass != "" {
			t := current.RotationStrategyClass
			if strings.HasSuffix(t, "Strategy") {
				t = t + "Config"
			}
			current.RotationStrategyConfig["type"] = t
		}
	}

	if current.RetentionStrategyClass == "" {
		current.RetentionStrategyClass = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
	}
	if current.RetentionStrategyConfig == nil || len(current.RetentionStrategyConfig) == 0 {
		current.RetentionStrategyConfig = map[string]any{
			"type":                  "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategyConfig",
			"max_number_of_indices": 20,
		}
	}
	if current.RetentionStrategyConfig != nil {
		if _, ok := current.RetentionStrategyConfig["type"]; !ok && current.RetentionStrategyClass != "" {
			t := current.RetentionStrategyClass
			if strings.HasSuffix(t, "Strategy") {
				t = t + "Config"
			}
			current.RetentionStrategyConfig["type"] = t
		}
	}

	// Нормализация значений по умолчанию
	if current.FieldTypeRefreshInterval == 0 {
		current.FieldTypeRefreshInterval = 5000
	}
	if current.IndexAnalyzer == "" {
		current.IndexAnalyzer = "standard"
	}
	if current.IndexOptimizationMaxNumSegments == 0 {
		current.IndexOptimizationMaxNumSegments = 1
	}

	// Формируем полное тело запроса со всеми полями
	// Важно: используем текущий объект целиком для PUT
	resp, err := c.doRequest("PUT", path, current)
	if err != nil {
		return nil, err
	}
	var out IndexSet
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeleteIndexSet(id string) error {
	// Начиная с Graylog 5 API стабильно доступен под /api/system/indices/index_sets/{id}
	path := fmt.Sprintf("/api/system/indices/index_sets/%s", id)
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
	// Унифицированный путь для всех версий
	path := "/api/system/pipelines/pipeline"
	resp, err := c.doRequest("POST", path, p)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) GetPipeline(id string) (*Pipeline, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdatePipeline(id string, p *Pipeline) (*Pipeline, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
	resp, err := c.doRequest("PUT", path, p)
	if err != nil {
		return nil, err
	}
	var out Pipeline
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) DeletePipeline(id string) error {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/system/pipelines/pipeline/%s", id)
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
	// Graylog 7.x использует Views API вместо legacy /dashboards
	if c.APIVersion == APIV7 {
		// Попробуем несколько безопасных вариантов создания дашборда через Views API
		variants := []struct {
			url  string
			body map[string]any
		}{
			{"/api/views", map[string]any{"type": "DASHBOARD", "title": d.Title, "summary": d.Description}},
			{"/api/views", map[string]any{"title": d.Title, "summary": d.Description, "type": "dashboard"}},
			{"/api/views/dashboards", map[string]any{"title": d.Title, "summary": d.Description}},
		}
		for _, v := range variants {
			if resp, err := c.doRequest("POST", v.url, v.body); err == nil {
				var out Dashboard
				// возможные ответы: {id,...} или {view:{id,...}}
				var aux map[string]any
				if json.Unmarshal(resp, &aux) == nil {
					if id, ok := aux["id"].(string); ok && id != "" {
						out.ID = id
						out.Title = d.Title
						out.Description = d.Description
						return &out, nil
					}
					if view, ok := aux["view"].(map[string]any); ok {
						if id, ok := view["id"].(string); ok && id != "" {
							out.ID = id
							out.Title = d.Title
							out.Description = d.Description
							return &out, nil
						}
					}
				}
				// попытка распаковать напрямую
				_ = json.Unmarshal(resp, &out)
				if out.ID != "" {
					return &out, nil
				}
			}
		}
		// Fallback: если Views API не сработал (возможно, сервер старше), пробуем legacy пути
		// v6: /api/dashboards, v5: /dashboards
		for _, path := range []string{"/api/dashboards", "/dashboards"} {
			if resp, err := c.doRequest("POST", path, d); err == nil {
				var out Dashboard
				if json.Unmarshal(resp, &out) == nil && out.ID != "" {
					return &out, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to create dashboard via Views API and legacy endpoints")
	}
	// legacy путь для v5/v6
	path := "/dashboards"
	if c.APIVersion == APIV6 {
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
	if c.APIVersion == APIV7 {
		// Views API
		resp, err := c.doRequest("GET", fmt.Sprintf("/api/views/%s", id), nil)
		if err != nil {
			return nil, err
		}
		// распарсим гибко
		var out Dashboard
		var aux map[string]any
		if json.Unmarshal(resp, &aux) == nil {
			// чаще всего поле title в корне или во вложенном view
			if title, ok := aux["title"].(string); ok {
				out.Title = title
			}
			if summary, ok := aux["summary"].(string); ok {
				out.Description = summary
			}
			out.ID = id
			if view, ok := aux["view"].(map[string]any); ok {
				if title, ok := view["title"].(string); ok {
					out.Title = title
				}
				if summary, ok := view["summary"].(string); ok {
					out.Description = summary
				}
			}
			return &out, nil
		}
		_ = json.Unmarshal(resp, &out)
		if out.ID == "" {
			out.ID = id
		}
		return &out, nil
	}
	// legacy
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 {
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
	if c.APIVersion == APIV7 {
		body := map[string]any{"title": d.Title}
		if d.Description != "" {
			body["summary"] = d.Description
		}
		if _, err := c.doRequest("PUT", fmt.Sprintf("/api/views/%s", id), body); err != nil {
			return nil, err
		}
		// перечитать
		return c.GetDashboard(id)
	}
	// legacy
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 {
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
	if c.APIVersion == APIV7 {
		_, err := c.doRequest("DELETE", fmt.Sprintf("/api/views/%s", id), nil)
		return err
	}
	path := fmt.Sprintf("/dashboards/%s", id)
	if c.APIVersion == APIV6 {
		path = fmt.Sprintf("/api/dashboards/%s", id)
	}
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

func (c *Client) ListDashboards() ([]Dashboard, error) {
	if c.APIVersion == APIV7 {
		resp, err := c.doRequest("GET", "/api/views", nil)
		if err != nil {
			return nil, err
		}
		// ожидаем список представлений, отфильтруем по типу DASHBOARD
		var res []Dashboard
		var aux map[string]any
		if json.Unmarshal(resp, &aux) == nil {
			if list, ok := aux["views"].([]any); ok {
				for _, it := range list {
					if m, ok := it.(map[string]any); ok {
						if t, _ := m["type"].(string); strings.ToUpper(t) == "DASHBOARD" || strings.ToLower(t) == "dashboard" {
							dd := Dashboard{}
							if id, _ := m["id"].(string); id != "" {
								dd.ID = id
							}
							if title, _ := m["title"].(string); title != "" {
								dd.Title = title
							}
							if summary, _ := m["summary"].(string); summary != "" {
								dd.Description = summary
							}
							res = append(res, dd)
						}
					}
				}
				return res, nil
			}
		}
		return res, nil
	}
	// legacy
	path := "/dashboards"
	if c.APIVersion == APIV6 {
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
	// Унифицированный путь для всех версий
	path := "/api/events/definitions"
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
	// Сформируем базовый payload для запроса с учётом особенностей v5
	var baseBody any = ed
	if c.APIVersion == APIV5 {
		// GL5 expects snake_case fields key_spec/notification_settings
		baseBody = map[string]any{
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
	}

	// Для устойчивости: пробуем оба варианта тела запроса вне зависимости от детекции версии,
	// меняя порядок приоритетов в зависимости от предположения о версии.
	tryBodies := []any{
		map[string]any{"entity": baseBody}, // v7-подобный вариант
		baseBody,                           // legacy/plain вариант
	}
	if c.APIVersion == APIV5 || c.APIVersion == APIV6 {
		tryBodies = []any{baseBody, map[string]any{"entity": baseBody}}
	}
	var lastResp []byte
	for i, b := range tryBodies {
		resp, err := c.doRequest("POST", path, b)
		if err != nil {
			lastResp = []byte(err.Error())
			if i+1 < len(tryBodies) {
				continue
			}
			return nil, err
		}
		var out EventDefinition
		_ = json.Unmarshal(resp, &out)
		return &out, nil
	}
	return nil, fmt.Errorf("failed to create event definition: unexpected response %s", string(lastResp))
}

func (c *Client) GetEventDefinition(id string) (*EventDefinition, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/events/definitions/%s", id)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out EventDefinition
	_ = json.Unmarshal(resp, &out)
	return &out, nil
}

func (c *Client) UpdateEventDefinition(id string, ed *EventDefinition) (*EventDefinition, error) {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/events/definitions/%s", id)
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
	// Унифицированный путь для всех версий
	path = fmt.Sprintf("/api/events/definitions/%s", id)
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
	// Для v7 используется CreateEntityRequest с обёрткой entity
	if c.APIVersion == APIV7 {
		// entity без поля type (см. ошибку маппинга), только title/description/config
		// Для v7 конфиг email чаще ожидает snake_case ключи — оставим как передано пользователем
		cfg := map[string]any{}
		for k, v := range n.Config {
			cfg[k] = v
		}
		// Для email-уведомлений в v7 требуется явный тип конфигурации
		if n.Type == "email" {
			if _, ok := cfg["type"]; !ok {
				// На разных сборках встречаются значения вида "email-notification-v1" или "org.graylog.events.notifications.types.EmailEventNotificationConfig"
				// Попробуем более короткий вариант, а при ошибке сервер вернёт конкретный ожидаемый тип
				cfg["type"] = "email-notification-v1"
			}
		}
		entity := map[string]any{
			"title":       n.Title,
			"description": n.Description,
			"config":      cfg,
		}
		body := map[string]any{
			// В Graylog 7 CreateEntityRequest для notifications ожидает только { entity, share_request? }
			"entity": entity,
		}
		resp, err := c.doRequest("POST", path, body)
		if err != nil {
			// Фолбэк: попробовать без обёртки (некоторые сборки v7 принимают legacy форму)
			if strings.Contains(err.Error(), "400") || strings.Contains(err.Error(), "RequestError") {
				if r2, e2 := c.doRequest("POST", path, n); e2 == nil {
					var out2 EventNotification
					_ = json.Unmarshal(r2, &out2)
					return &out2, nil
				}
			}
			return nil, err
		}
		var out EventNotification
		_ = json.Unmarshal(resp, &out)
		// Нормализуем тип: если в config.type присутствует email — проставим Type="email"
		if out.Type == "" {
			var aux map[string]any
			if json.Unmarshal(resp, &aux) == nil {
				if cfg, ok := aux["config"].(map[string]any); ok {
					if t, _ := cfg["type"].(string); strings.Contains(strings.ToLower(t), "email") {
						out.Type = "email"
					}
				}
			}
		}
		if out.ID == "" {
			// Возможный ответ-обёртка {"id":"..."}
			var aux map[string]any
			if json.Unmarshal(resp, &aux) == nil {
				if id, ok := aux["id"].(string); ok {
					out.ID = id
				}
			}
		}
		return &out, nil
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
	// Нормализуем тип на чтении
	if out.Type == "" {
		var aux map[string]any
		if json.Unmarshal(resp, &aux) == nil {
			if cfg, ok := aux["config"].(map[string]any); ok {
				if t, _ := cfg["type"].(string); strings.Contains(strings.ToLower(t), "email") {
					out.Type = "email"
				}
			}
		}
	}
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

// ---- Views (read-only listing used for governance/data sources) ----

type View struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ListViews attempts to list Views across different Graylog versions/images.
// Some images may not expose Views APIs; in that case, the function returns an empty slice and no error
// after probing known endpoints, so read-only data sources can degrade gracefully.
func (c *Client) ListViews() ([]View, error) {
	// Try a set of known endpoints; keep it lightweight
	paths := []string{
		"/api/views",
		"/views",
	}
	var lastErr error
	for _, p := range paths {
		resp, err := c.doRequest("GET", p, nil)
		if err != nil {
			// If endpoint is missing (404) or method not allowed (405), try next path
			var ge *GraylogError
			if errors.As(err, &ge) && (ge.Status == 404 || ge.Status == 405) {
				lastErr = err
				continue
			}
			// Other errors are considered fatal for this path; try next but remember last
			lastErr = err
			continue
		}
		// Try common wrappers first
		var wrap struct {
			Views    []View `json:"views"`
			Data     []View `json:"data"`
			Elements []View `json:"elements"`
			Items    []View `json:"items"`
		}
		if err := json.Unmarshal(resp, &wrap); err == nil {
			switch {
			case wrap.Views != nil:
				return wrap.Views, nil
			case wrap.Data != nil:
				return wrap.Data, nil
			case wrap.Elements != nil:
				return wrap.Elements, nil
			case wrap.Items != nil:
				return wrap.Items, nil
			}
		}
		// Direct array
		var arr []View
		if err := json.Unmarshal(resp, &arr); err == nil && arr != nil {
			return arr, nil
		}
		// Generic object with a "views" field of various shapes
		var aux map[string]any
		if err := json.Unmarshal(resp, &aux); err == nil && aux != nil {
			if v, ok := aux["views"]; ok {
				switch t := v.(type) {
				case []any:
					out := make([]View, 0, len(t))
					for _, it := range t {
						b, _ := json.Marshal(it)
						var vv View
						if json.Unmarshal(b, &vv) == nil {
							out = append(out, vv)
						}
					}
					return out, nil
				case map[string]any:
					out := make([]View, 0, len(t))
					for _, it := range t {
						b, _ := json.Marshal(it)
						var vv View
						if json.Unmarshal(b, &vv) == nil {
							out = append(out, vv)
						}
					}
					return out, nil
				}
			}
		}
		// If we reached here, response shape is unexpected — try next path
		lastErr = errors.New("unexpected views response format")
	}
	// If all paths failed with 404/405 (feature not present), return empty list gracefully
	if lastErr != nil {
		var ge *GraylogError
		if errors.As(lastErr, &ge) && (ge.Status == 404 || ge.Status == 405) {
			return []View{}, nil
		}
	}
	if lastErr == nil {
		return []View{}, nil
	}
	return nil, lastErr
}

// ---- Users ----

type User struct {
	ID               string   `json:"id,omitempty"`
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
	// Унифицированный путь для всех версий
	path := "/api/users"
	// Graylog 5/6/7 на CreateUser обычно ожидает first_name/last_name вместо full_name
	var body any = u // default
	if c.APIVersion == APIV5 || c.APIVersion == APIV6 || c.APIVersion == APIV7 {
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
		// 'disabled' may be unsupported in CreateUserRequest; применим через Update при необходимости
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
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/users/%s", username)
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
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/users/%s", username)
	// Для v7 многие операции Update требуют ObjectId в пути
	if c.APIVersion == APIV7 {
		// Попробуем получить ID пользователя и, если он имеется, использовать маршрут по ID
		current, _ := c.GetUser(username)
		if current != nil && current.ID != "" {
			path = fmt.Sprintf("/api/users/%s", current.ID)
		}
		payload := map[string]any{}
		if u.FullName != "" {
			// В некоторых сборках v7 используются camelCase ключи
			payload["fullName"] = u.FullName
		}
		payload["disabled"] = u.Disabled
		// роли и email обновляем, если заданы (некоторые сборки v7 принимают эти поля)
		if len(u.Roles) > 0 {
			payload["roles"] = u.Roles
		}
		if u.Email != "" {
			payload["email"] = u.Email
		}
		if _, err := c.doRequest("PUT", path, payload); err != nil {
			return nil, err
		}
		// Синхронизируем disabled явными эндпоинтами, если доступны
		if current != nil && current.ID != "" {
			if u.Disabled {
				_, _ = c.doRequest("POST", fmt.Sprintf("/api/users/%s/disable", current.ID), nil)
			} else {
				_, _ = c.doRequest("POST", fmt.Sprintf("/api/users/%s/enable", current.ID), nil)
			}
		}
		// Фолбэк: попробуем PUT по username со snake_case, если изменения не применятся
		_, _ = c.doRequest("PUT", fmt.Sprintf("/api/users/%s", username), map[string]any{
			"full_name": u.FullName,
			"disabled":  u.Disabled,
		})
		// Вернуть актуальное состояние
		return c.GetUser(username)
	}
	// Копия без пароля для основного апдейта (v5/v6)
	body := *u
	body.Password = ""
	_, err := c.doRequest("PUT", path, &body)
	if err != nil {
		return nil, err
	}
	// Если задан пароль — выполнить отдельный вызов смены пароля
	if u.Password != "" {
		ppath := fmt.Sprintf("/api/users/%s/password", username)
		_, err := c.doRequest("PUT", ppath, map[string]string{"password": u.Password})
		if err != nil {
			return nil, err
		}
	}
	// Вернуть актуальное состояние
	return c.GetUser(username)
}

func (c *Client) DeleteUser(username string) error {
	// Унифицированный путь для всех версий
	path := fmt.Sprintf("/api/users/%s", username)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// ListUsers returns all users. Graylog may return either a wrapped object
// like {"users": [...]} or a raw array; support both. Newer versions may
// include ObjectId in field `id`.
func (c *Client) ListUsers() ([]User, error) {
	path := "/api/users"
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Try wrapped
	var wrap struct {
		Users []User `json:"users"`
	}
	if err := json.Unmarshal(resp, &wrap); err == nil && wrap.Users != nil {
		return wrap.Users, nil
	}
	// Raw array
	var arr []User
	if err := json.Unmarshal(resp, &arr); err == nil && arr != nil {
		// Ensure password field is blanked just in case
		for i := range arr {
			arr[i].Password = ""
		}
		return arr, nil
	}
	// Fallback for map formats
	var anyMap map[string]any
	if err := json.Unmarshal(resp, &anyMap); err == nil {
		if v, ok := anyMap["users"]; ok {
			switch t := v.(type) {
			case []any:
				out := make([]User, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var u User
					if err := json.Unmarshal(b, &u); err == nil {
						u.Password = ""
						out = append(out, u)
					}
				}
				return out, nil
			case map[string]any:
				out := make([]User, 0, len(t))
				for _, it := range t {
					b, _ := json.Marshal(it)
					var u User
					if err := json.Unmarshal(b, &u); err == nil {
						u.Password = ""
						out = append(out, u)
					}
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("unexpected users response format")
}
