package provider

import (
	"context"
	"fmt"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type openSearchSnapshotRepositoryResource struct{ client *client.Client }

type osRepoFSSettings struct {
	Location               types.String `tfsdk:"location"`
	Compress               types.Bool   `tfsdk:"compress"`
	MaxSnapshotBytesPerSec types.String `tfsdk:"max_snapshot_bytes_per_sec"`
	MaxRestoreBytesPerSec  types.String `tfsdk:"max_restore_bytes_per_sec"`
	ChunkSize              types.String `tfsdk:"chunk_size"`
}

type osRepoS3Settings struct {
	Bucket          types.String `tfsdk:"bucket"`
	Region          types.String `tfsdk:"region"`
	Endpoint        types.String `tfsdk:"endpoint"`
	BasePath        types.String `tfsdk:"base_path"`
	Protocol        types.String `tfsdk:"protocol"`
	PathStyleAccess types.Bool   `tfsdk:"path_style_access"`
	ReadOnly        types.Bool   `tfsdk:"read_only"`
	AccessKey       types.String `tfsdk:"access_key"`
	SecretKey       types.String `tfsdk:"secret_key"`
	SessionToken    types.String `tfsdk:"session_token"`
}

type openSearchSnapshotRepositoryModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Settings types.Map    `tfsdk:"settings"`

	FS *osRepoFSSettings `tfsdk:"fs_settings"`
	S3 *osRepoS3Settings `tfsdk:"s3_settings"`
}

func NewOpenSearchSnapshotRepositoryResource() resource.Resource {
	return &openSearchSnapshotRepositoryResource{}
}

func (r *openSearchSnapshotRepositoryResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_opensearch_snapshot_repository"
}

func (r *openSearchSnapshotRepositoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages OpenSearch Snapshot Repository (supports generic type+settings; typed blocks for fs and s3).",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, Description: "Resource ID (same as name)"},
			"name": schema.StringAttribute{Required: true, Description: "Repository name"},
			"type": schema.StringAttribute{Required: true, Description: "Repository type (e.g., fs, s3)"},
			"settings": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Generic settings map for repository type. Mutually exclusive with fs_settings/s3_settings.",
			},
		},
		Blocks: map[string]schema.Block{
			"fs_settings": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"location":                   schema.StringAttribute{Optional: true, Description: "Filesystem path for snapshots (must be in path.repo)"},
					"compress":                   schema.BoolAttribute{Optional: true},
					"max_snapshot_bytes_per_sec": schema.StringAttribute{Optional: true},
					"max_restore_bytes_per_sec":  schema.StringAttribute{Optional: true},
					"chunk_size":                 schema.StringAttribute{Optional: true},
				},
				Description: "Typed settings for fs repository. Conflicts with generic settings and s3_settings.",
			},
			"s3_settings": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"bucket":            schema.StringAttribute{Optional: true},
					"region":            schema.StringAttribute{Optional: true},
					"endpoint":          schema.StringAttribute{Optional: true},
					"base_path":         schema.StringAttribute{Optional: true},
					"protocol":          schema.StringAttribute{Optional: true},
					"path_style_access": schema.BoolAttribute{Optional: true},
					"read_only":         schema.BoolAttribute{Optional: true},
					"access_key":        schema.StringAttribute{Optional: true, Sensitive: true},
					"secret_key":        schema.StringAttribute{Optional: true, Sensitive: true},
					"session_token":     schema.StringAttribute{Optional: true, Sensitive: true},
				},
				Description: "Typed settings for s3 repository. Conflicts with generic settings and fs_settings.",
			},
		},
	}
}

func (r *openSearchSnapshotRepositoryResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *openSearchSnapshotRepositoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data openSearchSnapshotRepositoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	rtype := data.Type.ValueString()
	if name == "" || rtype == "" {
		resp.Diagnostics.AddError("Invalid input", "name and type are required")
		return
	}
	// Validate mutual exclusivity of settings representations
	hasGeneric := !data.Settings.IsNull() && !data.Settings.IsUnknown()
	hasFS := data.FS != nil
	hasS3 := data.S3 != nil
	if (hasFS || hasS3) && hasGeneric {
		resp.Diagnostics.AddError("Invalid configuration", "'settings' conflicts with 'fs_settings'/'s3_settings'. Use either a generic map or a typed block.")
		return
	}
	if hasFS && hasS3 {
		resp.Diagnostics.AddError("Invalid configuration", "'fs_settings' conflicts with 's3_settings'. Only one typed block is allowed.")
		return
	}
	if rtype == "fs" && hasS3 {
		resp.Diagnostics.AddError("Invalid configuration", "type 'fs' conflicts with provided 's3_settings'.")
		return
	}
	if rtype == "s3" && hasFS {
		resp.Diagnostics.AddError("Invalid configuration", "type 's3' conflicts with provided 'fs_settings'.")
		return
	}
	settings := r.collectSettings(&data)
	if rtype == "fs" {
		if _, ok := settings["location"]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("fs_settings").AtName("location"), "Missing required setting", "fs repository requires 'location' setting")
			return
		}
	} else if rtype == "s3" {
		if _, ok := settings["bucket"]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("s3_settings").AtName("bucket"), "Missing required setting", "s3 repository requires 'bucket' setting")
			return
		}
	}
	if err := r.client.WithContext(ctx).OSUpsertSnapshotRepository(name, rtype, settings); err != nil {
		resp.Diagnostics.AddError("Error creating snapshot repository", err.Error())
		return
	}
	data.ID = types.StringValue(name)
	// Preserve user's representation on create
	r.populateStateFromSettings(&data, rtype, settings, hasGeneric)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *openSearchSnapshotRepositoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data openSearchSnapshotRepositoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Preserve previous sensitive values (if present in state) to avoid plan drift
	var prevAccess, prevSecret, prevToken types.String
	hadS3Block := data.S3 != nil
	if hadS3Block {
		prevAccess = data.S3.AccessKey
		prevSecret = data.S3.SecretKey
		prevToken = data.S3.SessionToken
	}
	name := data.Name.ValueString()
	if name == "" {
		if !data.ID.IsNull() && !data.ID.IsUnknown() {
			name = data.ID.ValueString()
			data.Name = types.StringValue(name)
		}
	}
	rtype, settings, err := r.client.WithContext(ctx).OSGetSnapshotRepository(name)
	if err != nil {
		if err == client.ErrNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading snapshot repository", err.Error())
		return
	}
	data.Type = types.StringValue(rtype)
	// Preserve user's representation: if generic map was used in state — keep it; else try typed blocks.
	if !data.Settings.IsNull() && !data.Settings.IsUnknown() {
		// Convert map[string]any to map[string]string for state
		m := map[string]string{}
		for k, v := range settings {
			m[k] = fmt.Sprintf("%v", v)
		}
		mv, _ := types.MapValueFrom(ctx, types.StringType, m)
		data.Settings = mv
	} else if data.FS != nil && (rtype == "fs" || data.Type.ValueString() == "fs") {
		r.applyToFSBlock(&data, settings)
		// Ensure settings attribute is null to avoid conflicts
		data.Settings = types.MapNull(types.StringType)
	} else if data.S3 != nil && (rtype == "s3" || data.Type.ValueString() == "s3") {
		r.applyToS3Block(&data, settings, true)
		// Re-apply sensitive values from previous state to avoid perpetual diffs
		if hadS3Block {
			if !prevAccess.IsNull() && !prevAccess.IsUnknown() && prevAccess.ValueString() != "" {
				data.S3.AccessKey = prevAccess
			}
			if !prevSecret.IsNull() && !prevSecret.IsUnknown() && prevSecret.ValueString() != "" {
				data.S3.SecretKey = prevSecret
			}
			if !prevToken.IsNull() && !prevToken.IsUnknown() && prevToken.ValueString() != "" {
				data.S3.SessionToken = prevToken
			}
		}
		data.Settings = types.MapNull(types.StringType)
	} else {
		// Default to generic settings map
		m := map[string]string{}
		for k, v := range settings {
			m[k] = fmt.Sprintf("%v", v)
		}
		mv, _ := types.MapValueFrom(ctx, types.StringType, m)
		data.Settings = mv
		data.FS = nil
		data.S3 = nil
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *openSearchSnapshotRepositoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data openSearchSnapshotRepositoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	rtype := data.Type.ValueString()
	if name == "" || rtype == "" {
		resp.Diagnostics.AddError("Invalid input", "name and type are required")
		return
	}
	// Validate mutual exclusivity of settings representations
	hasGeneric := !data.Settings.IsNull() && !data.Settings.IsUnknown()
	hasFS := data.FS != nil
	hasS3 := data.S3 != nil
	if (hasFS || hasS3) && hasGeneric {
		resp.Diagnostics.AddError("Invalid configuration", "'settings' conflicts with 'fs_settings'/'s3_settings'. Use either a generic map or a typed block.")
		return
	}
	if hasFS && hasS3 {
		resp.Diagnostics.AddError("Invalid configuration", "'fs_settings' conflicts with 's3_settings'. Only one typed block is allowed.")
		return
	}
	if rtype == "fs" && hasS3 {
		resp.Diagnostics.AddError("Invalid configuration", "type 'fs' conflicts with provided 's3_settings'.")
		return
	}
	if rtype == "s3" && hasFS {
		resp.Diagnostics.AddError("Invalid configuration", "type 's3' conflicts with provided 'fs_settings'.")
		return
	}
	settings := r.collectSettings(&data)
	if rtype == "fs" {
		if _, ok := settings["location"]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("fs_settings").AtName("location"), "Missing required setting", "fs repository requires 'location' setting")
			return
		}
	} else if rtype == "s3" {
		if _, ok := settings["bucket"]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("s3_settings").AtName("bucket"), "Missing required setting", "s3 repository requires 'bucket' setting")
			return
		}
	}
	if err := r.client.WithContext(ctx).OSUpsertSnapshotRepository(name, rtype, settings); err != nil {
		resp.Diagnostics.AddError("Error updating snapshot repository", err.Error())
		return
	}
	data.ID = types.StringValue(name)
	r.populateStateFromSettings(&data, rtype, settings, hasGeneric)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *openSearchSnapshotRepositoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data openSearchSnapshotRepositoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	if name == "" && !data.ID.IsNull() && !data.ID.IsUnknown() {
		name = data.ID.ValueString()
	}
	if name == "" {
		return
	}
	if err := r.client.WithContext(ctx).OSDeleteSnapshotRepository(name); err != nil {
		resp.Diagnostics.AddError("Error deleting snapshot repository", err.Error())
		return
	}
}

func (r *openSearchSnapshotRepositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers
func (r *openSearchSnapshotRepositoryResource) collectSettings(m *openSearchSnapshotRepositoryModel) map[string]any {
	// Prefer generic settings if provided
	if !m.Settings.IsNull() && !m.Settings.IsUnknown() {
		// Convert to map[string]any from map[string]string
		res := map[string]any{}
		var mm map[string]string
		// best effort
		_ = m.Settings.ElementsAs(context.Background(), &mm, false)
		for k, v := range mm {
			res[k] = v
		}
		return res
	}
	if m.FS != nil {
		s := map[string]any{}
		if v := m.FS.Location.ValueString(); v != "" {
			s["location"] = v
		}
		if !m.FS.Compress.IsNull() && !m.FS.Compress.IsUnknown() {
			s["compress"] = m.FS.Compress.ValueBool()
		}
		if v := m.FS.MaxSnapshotBytesPerSec.ValueString(); v != "" {
			s["max_snapshot_bytes_per_sec"] = v
		}
		if v := m.FS.MaxRestoreBytesPerSec.ValueString(); v != "" {
			s["max_restore_bytes_per_sec"] = v
		}
		if v := m.FS.ChunkSize.ValueString(); v != "" {
			s["chunk_size"] = v
		}
		return s
	}
	if m.S3 != nil {
		s := map[string]any{}
		if v := m.S3.Bucket.ValueString(); v != "" {
			s["bucket"] = v
		}
		if v := m.S3.Region.ValueString(); v != "" {
			s["region"] = v
		}
		if v := m.S3.Endpoint.ValueString(); v != "" {
			s["endpoint"] = v
		}
		if v := m.S3.BasePath.ValueString(); v != "" {
			s["base_path"] = v
		}
		if v := m.S3.Protocol.ValueString(); v != "" {
			s["protocol"] = v
		}
		if !m.S3.PathStyleAccess.IsNull() && !m.S3.PathStyleAccess.IsUnknown() {
			s["path_style_access"] = m.S3.PathStyleAccess.ValueBool()
		}
		if !m.S3.ReadOnly.IsNull() && !m.S3.ReadOnly.IsUnknown() {
			s["read_only"] = m.S3.ReadOnly.ValueBool()
		}
		// Sensitive keys are sent but never reflected back into state on Read
		if v := m.S3.AccessKey.ValueString(); v != "" {
			s["access_key"] = v
		}
		if v := m.S3.SecretKey.ValueString(); v != "" {
			s["secret_key"] = v
		}
		if v := m.S3.SessionToken.ValueString(); v != "" {
			s["session_token"] = v
		}
		return s
	}
	return map[string]any{}
}

func (r *openSearchSnapshotRepositoryResource) populateStateFromSettings(m *openSearchSnapshotRepositoryModel, rtype string, settings map[string]any, preferGeneric bool) {
	// If user used generic settings representation, keep it
	if preferGeneric {
		sm := map[string]string{}
		for k, v := range settings {
			sm[k] = fmt.Sprintf("%v", v)
		}
		mv, _ := types.MapValueFrom(context.Background(), types.StringType, sm)
		m.Settings = mv
		m.FS = nil
		m.S3 = nil
		return
	}
	switch rtype {
	case "fs":
		if m.FS == nil {
			m.FS = &osRepoFSSettings{}
		}
		r.applyToFSBlock(m, settings)
		m.Settings = types.MapNull(types.StringType)
		m.S3 = nil
	case "s3":
		if m.S3 == nil {
			m.S3 = &osRepoS3Settings{}
		}
		r.applyToS3Block(m, settings, false)
		m.Settings = types.MapNull(types.StringType)
		m.FS = nil
	default:
		// generic fallback
		sm := map[string]string{}
		for k, v := range settings {
			sm[k] = fmt.Sprintf("%v", v)
		}
		mv, _ := types.MapValueFrom(context.Background(), types.StringType, sm)
		m.Settings = mv
		m.FS = nil
		m.S3 = nil
	}
}

func (r *openSearchSnapshotRepositoryResource) applyToFSBlock(m *openSearchSnapshotRepositoryModel, settings map[string]any) {
	if m.FS == nil {
		m.FS = &osRepoFSSettings{}
	}
	if v, ok := settings["location"].(string); ok {
		m.FS.Location = types.StringValue(v)
	}
	if v, ok := settings["compress"].(bool); ok {
		m.FS.Compress = types.BoolValue(v)
	}
	if v, ok := settings["max_snapshot_bytes_per_sec"].(string); ok {
		m.FS.MaxSnapshotBytesPerSec = types.StringValue(v)
	}
	if v, ok := settings["max_restore_bytes_per_sec"].(string); ok {
		m.FS.MaxRestoreBytesPerSec = types.StringValue(v)
	}
	if v, ok := settings["chunk_size"].(string); ok {
		m.FS.ChunkSize = types.StringValue(v)
	}
}

func (r *openSearchSnapshotRepositoryResource) applyToS3Block(m *openSearchSnapshotRepositoryModel, settings map[string]any, zeroSensitive bool) {
	if m.S3 == nil {
		m.S3 = &osRepoS3Settings{}
	}
	if v, ok := settings["bucket"].(string); ok {
		m.S3.Bucket = types.StringValue(v)
	}
	if v, ok := settings["region"].(string); ok {
		m.S3.Region = types.StringValue(v)
	}
	if v, ok := settings["endpoint"].(string); ok {
		m.S3.Endpoint = types.StringValue(v)
	}
	if v, ok := settings["base_path"].(string); ok {
		m.S3.BasePath = types.StringValue(v)
	}
	if v, ok := settings["protocol"].(string); ok {
		m.S3.Protocol = types.StringValue(v)
	}
	if v, ok := settings["path_style_access"].(bool); ok {
		m.S3.PathStyleAccess = types.BoolValue(v)
	}
	if v, ok := settings["read_only"].(bool); ok {
		m.S3.ReadOnly = types.BoolValue(v)
	}
	// Sensitive keys are never set back from server response. If requested, zero them.
	if zeroSensitive {
		m.S3.AccessKey = types.StringNull()
		m.S3.SecretKey = types.StringNull()
		m.S3.SessionToken = types.StringNull()
	}
}
