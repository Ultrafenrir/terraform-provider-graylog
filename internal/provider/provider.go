package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type graylogProvider struct{}

type graylogProviderModel struct {
	URL types.String `tfsdk:"url"`
	// Auth
	AuthMethod       types.String `tfsdk:"auth_method"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	APIToken         types.String `tfsdk:"api_token"`
	APITokenPassword types.String `tfsdk:"api_token_password"`
	BearerToken      types.String `tfsdk:"bearer_token"`
	Token            types.String `tfsdk:"token"` // legacy base64
	// TLS/HTTP
	Insecure           types.Bool   `tfsdk:"insecure"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	CABundlePath       types.String `tfsdk:"ca_bundle"`
	ClientCertPath     types.String `tfsdk:"client_cert"`
	ClientKeyPath      types.String `tfsdk:"client_key"`
	Timeout            types.String `tfsdk:"timeout"`
	MaxRetries         types.Int64  `tfsdk:"max_retries"`
	RetryWait          types.String `tfsdk:"retry_wait"`
	LogLevel           types.String `tfsdk:"log_level"`

	// OpenSearch (optional)
	OpenSearchURL      types.String `tfsdk:"opensearch_url"`
	OpenSearchInsecure types.Bool   `tfsdk:"opensearch_insecure"`
}

func (p *graylogProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "graylog"
}

func (p *graylogProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Attributes: map[string]providerschema.Attribute{
			"url": providerschema.StringAttribute{Required: true},
			// Auth
			"auth_method":        providerschema.StringAttribute{Optional: true},
			"username":           providerschema.StringAttribute{Optional: true},
			"password":           providerschema.StringAttribute{Optional: true, Sensitive: true},
			"api_token":          providerschema.StringAttribute{Optional: true, Sensitive: true},
			"api_token_password": providerschema.StringAttribute{Optional: true, Sensitive: true},
			"bearer_token":       providerschema.StringAttribute{Optional: true, Sensitive: true},
			"token":              providerschema.StringAttribute{Optional: true, Sensitive: true, DeprecationMessage: "use username/password or api_token instead"},
			// TLS/HTTP
			"insecure":             providerschema.BoolAttribute{Optional: true, Description: "Alias of insecure_skip_verify. When true, TLS certificate verification is skipped (self-signed certs)."},
			"insecure_skip_verify": providerschema.BoolAttribute{Optional: true},
			"ca_bundle":            providerschema.StringAttribute{Optional: true},
			"client_cert":          providerschema.StringAttribute{Optional: true},
			"client_key":           providerschema.StringAttribute{Optional: true},
			"timeout":              providerschema.StringAttribute{Optional: true},
			"max_retries":          providerschema.Int64Attribute{Optional: true},
			"retry_wait":           providerschema.StringAttribute{Optional: true},
			"log_level":            providerschema.StringAttribute{Optional: true},
			// OpenSearch (optional)
			"opensearch_url":      providerschema.StringAttribute{Optional: true, Description: "Base URL of OpenSearch (for auxiliary features like snapshot repositories)"},
			"opensearch_insecure": providerschema.BoolAttribute{Optional: true, Description: "Skip TLS verification for OpenSearch HTTP client (shares transport with Graylog)"},
		},
	}
}

func (p *graylogProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data graylogProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Fallback to environment variables when attributes are unset in configuration
	// URL
	url := firstNonEmpty(getString(data.URL), os.Getenv("GRAYLOG_URL"))
	// Auth
	authMethod := firstNonEmpty(getString(data.AuthMethod), os.Getenv("GRAYLOG_AUTH_METHOD"))
	username := firstNonEmpty(getString(data.Username), os.Getenv("GRAYLOG_USERNAME"))
	password := firstNonEmpty(getString(data.Password), os.Getenv("GRAYLOG_PASSWORD"))
	apiToken := firstNonEmpty(getString(data.APIToken), os.Getenv("GRAYLOG_API_TOKEN"))
	apiTokenPassword := firstNonEmpty(getString(data.APITokenPassword), os.Getenv("GRAYLOG_API_TOKEN_PASSWORD"))
	bearerToken := firstNonEmpty(getString(data.BearerToken), os.Getenv("GRAYLOG_BEARER_TOKEN"))
	legacyToken := firstNonEmpty(getString(data.Token), os.Getenv("GRAYLOG_TOKEN"))

	// TLS/HTTP
	// Support both `insecure` and legacy `insecure_skip_verify` attributes; also GRAYLOG_INSECURE=1
	insecure := getBool(data.Insecure, os.Getenv("GRAYLOG_INSECURE") == "1") ||
		getBool(data.InsecureSkipVerify, os.Getenv("GRAYLOG_INSECURE") == "1")
	caBundle := firstNonEmpty(getString(data.CABundlePath), os.Getenv("GRAYLOG_CA_BUNDLE"))
	clientCert := firstNonEmpty(getString(data.ClientCertPath), os.Getenv("GRAYLOG_CLIENT_CERT"))
	clientKey := firstNonEmpty(getString(data.ClientKeyPath), os.Getenv("GRAYLOG_CLIENT_KEY"))
	timeoutStr := firstNonEmpty(getString(data.Timeout), os.Getenv("GRAYLOG_TIMEOUT"))
	retryWaitStr := firstNonEmpty(getString(data.RetryWait), os.Getenv("GRAYLOG_RETRY_WAIT"))
	maxRetries := getIntDefault(data.MaxRetries, os.Getenv("GRAYLOG_MAX_RETRIES"))
	logLevel := firstNonEmpty(getString(data.LogLevel), os.Getenv("GRAYLOG_LOG_LEVEL"))

	// OpenSearch
	osURL := firstNonEmpty(getString(data.OpenSearchURL), os.Getenv("OPENSEARCH_URL"))
	osInsecure := getBool(data.OpenSearchInsecure, os.Getenv("OPENSEARCH_INSECURE") == "1")

	// Parse durations
	var timeout time.Duration
	if timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}
	var retryWait time.Duration
	if retryWaitStr != "" {
		if d, err := time.ParseDuration(retryWaitStr); err == nil {
			retryWait = d
		}
	}

	// Validate auth_method (basic)
	switch authMethod {
	case "", "auto", "basic_userpass", "basic_token", "basic_legacy_b64", "bearer", "none":
	default:
		resp.Diagnostics.AddError("invalid auth_method", "supported: auto|basic_userpass|basic_token|basic_legacy_b64|bearer|none")
		return
	}

	opts := client.Options{
		AuthMethod:         authMethod,
		Username:           username,
		Password:           password,
		APIToken:           apiToken,
		APITokenPassword:   apiTokenPassword,
		BearerToken:        bearerToken,
		LegacyTokenB64:     legacyToken,
		InsecureSkipVerify: insecure || osInsecure,
		CABundlePath:       caBundle,
		ClientCertPath:     clientCert,
		ClientKeyPath:      clientKey,
		Timeout:            timeout,
		MaxRetries:         maxRetries,
		RetryWait:          retryWait,
		OpenSearchURL:      osURL,
	}

	c := client.NewWithOptions(url, opts)

	// tflog adapter with level filtering
	c.SetLogger(newTfLogger(logLevel))
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *graylogProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewStreamResource,
		NewInputResource,
		NewIndexSetResource,
		NewPipelineResource,
		NewDashboardResource,
		NewDashboardWidgetResource,
		NewAlertResource,
		NewEventNotificationResource,
		NewLDAPSettingResource,
		NewOutputResource,
		NewStreamOutputBindingResource,
		NewStreamPermissionResource,
		NewDashboardPermissionResource,
		NewRoleResource,
		NewUserResource,
		NewOpenSearchSnapshotRepositoryResource,
	}
}

func (p *graylogProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewStreamDataSource,
		NewStreamsListDataSource,
		NewViewsListDataSource,
		NewInputDataSource,
		NewInputsListDataSource,
		NewIndexSetDataSource,
		NewIndexSetDefaultDataSource,
		NewIndexSetsListDataSource,
		NewDashboardDataSource,
		NewDashboardsListDataSource,
		NewEventNotificationDataSource,
		NewEventNotificationsListDataSource,
		NewUserDataSource,
		NewUsersListDataSource,
		NewLDAPGroupMembersDataSource,
	}
}

func New() provider.Provider { return &graylogProvider{} }

// ===== Вспомогательные функции/типы =====

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func getString(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func getBool(v types.Bool, fallback bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return fallback
	}
	return v.ValueBool()
}

func getIntDefault(v types.Int64, env string) int {
	if !v.IsNull() && !v.IsUnknown() {
		return int(v.ValueInt64())
	}
	if env == "" {
		return 0
	}
	// простая попытка распарсить
	var n int
	_, _ = fmt.Sscanf(env, "%d", &n)
	return n
}

// tfLogger — адаптер к клиентскому интерфейсу логгера с выводом в tflog
type tfLogger struct {
	level string // debug|info|warn|error
}

func newTfLogger(level string) *tfLogger { return &tfLogger{level: level} }

func (l *tfLogger) allowDebug() bool { return l.level == "" || l.level == "debug" }
func (l *tfLogger) allowInfo() bool  { return l.allowDebug() || l.level == "info" }
func (l *tfLogger) allowWarn() bool  { return l.allowInfo() || l.level == "warn" }

func (l *tfLogger) Debug(ctx context.Context, msg string, fields client.Fields) {
	if !l.allowDebug() {
		return
	}
	tflog.Debug(ctx, msg, map[string]any(fields))
}
func (l *tfLogger) Info(ctx context.Context, msg string, fields client.Fields) {
	if !l.allowInfo() {
		return
	}
	tflog.Info(ctx, msg, map[string]any(fields))
}
func (l *tfLogger) Warn(ctx context.Context, msg string, fields client.Fields) {
	if !l.allowWarn() {
		return
	}
	tflog.Warn(ctx, msg, map[string]any(fields))
}
func (l *tfLogger) Error(ctx context.Context, msg string, fields client.Fields) {
	tflog.Error(ctx, msg, map[string]any(fields))
}
