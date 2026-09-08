package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type clusterConfigResource struct{ client *client.Client }

type clusterConfigModel struct {
	ID         types.String   `tfsdk:"id"`
	Class      types.String   `tfsdk:"class"`
	ConfigJSON types.String   `tfsdk:"config_json"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

func NewClusterConfigResource() resource.Resource { return &clusterConfigResource{} }

func (r *clusterConfigResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_cluster_config"
}

func (r *clusterConfigResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Description: "Manages a document in Graylog's cluster configuration store (`/system/cluster_config/{class}`). " +
			"Each document is keyed by the fully qualified name of the Graylog configuration class that reads it, " +
			"which lets a single resource manage any cluster-wide setting that has no dedicated resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Resource ID (identical to `class`).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"class": schema.StringAttribute{
				Required: true,
				Description: "Fully qualified name of the Graylog configuration class, for example " +
					"`org.graylog2.users.UserConfiguration`. The class must be resolvable by the server and must be " +
					"covered by the server's `safe_classes` setting (default `org.graylog.,org.graylog2.`); anything " +
					"else is rejected by Graylog. Changing the class replaces the resource, because a different " +
					"class is a different document.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config_json": schema.StringAttribute{
				Required: true,
				Description: "The configuration document, JSON-encoded. Graylog deserializes it into the target " +
					"class and rejects incomplete documents, so every field that class requires must be present — " +
					"this is a whole-document replace, not a patch. Key order and whitespace are not significant. " +
					"Only the keys present here take part in drift detection; defaults the server materializes " +
					"into the stored document are ignored.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *clusterConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

// validatedDocument parses the practitioner-supplied document and rejects
// anything that is not a JSON object. Graylog always deserializes the body
// into a class, so a top-level array or scalar can never be valid and is
// worth catching before the round trip.
func validatedDocument(raw string) (json.RawMessage, error) {
	if raw == "" {
		return nil, errors.New("config_json must not be empty")
	}
	var probe any
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, fmt.Errorf("config_json is not valid JSON: %w", err)
	}
	if _, ok := probe.(map[string]any); !ok {
		return nil, errors.New("config_json must be a JSON object")
	}
	return json.RawMessage(raw), nil
}

// isEncryptedValueSentinel reports whether v is the {"is_set": <bool>} object
// Graylog echoes in place of an EncryptedValue field. The shape is matched
// strictly so a practitioner's own object with an is_set key is left alone.
func isEncryptedValueSentinel(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok || len(obj) != 1 {
		return false
	}
	_, ok = obj["is_set"].(bool)
	return ok
}

// stripEncryptedValueSentinels removes EncryptedValue sentinels from a decoded
// document, recursing into nested objects. The sentinel is read-only: writing
// it back is rejected with "set_value must be a string and cannot be missing",
// so a document that carries one can never be applied.
func stripEncryptedValueSentinels(v any) {
	obj, ok := v.(map[string]any)
	if !ok {
		return
	}
	for key, value := range obj {
		if isEncryptedValueSentinel(value) {
			delete(obj, key)
			continue
		}
		stripEncryptedValueSentinels(value)
	}
}

// refreshClusterConfigDocument turns the server document into the value
// config_json should hold after a read.
//
// Graylog does not store every class verbatim. GeoIpResolverConfig, for one,
// echoes eight keys the practitioner never sent, among them the EncryptedValue
// sentinel {"is_set": false} that the server itself refuses on a write. So
// the echo is projected onto the keys present in the state document before
// it is compared: server-materialized defaults never show up as drift, while
// a managed key that changed server-side still does.
//
// Sentinels are dropped first. With a mask that makes no observable
// difference, but an imported resource has no mask yet and adopts the whole
// document; stripping keeps that document something that can be applied.
func refreshClusterConfigDocument(serverJSON, stateJSON string) (string, error) {
	server, err := decodeJSONPreservingNumbers(serverJSON)
	if err != nil {
		return "", fmt.Errorf("server returned a document that is not valid JSON: %w", err)
	}
	stripEncryptedValueSentinels(server)
	stripped, err := CanonicalizeJSONValue(server)
	if err != nil {
		return "", err
	}
	return ProjectAndCanonicalizeJSON(stripped, stateJSON)
}

// write pushes the planned document to Graylog. State keeps the
// practitioner's document rather than the server echo: Read compares only
// the keys the practitioner manages, so the two are equivalent for drift
// purposes, and writing back a re-serialized copy would risk "inconsistent
// result after apply" over nothing more than whitespace or key order.
func (r *clusterConfigResource) write(ctx context.Context, data *clusterConfigModel) error {
	doc, err := validatedDocument(data.ConfigJSON.ValueString())
	if err != nil {
		return err
	}
	if _, err := r.client.WithContext(ctx).UpdateClusterConfig(data.Class.ValueString(), doc); err != nil {
		return err
	}
	data.ID = types.StringValue(data.Class.ValueString())
	return nil
}

func (r *clusterConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data clusterConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	if err := r.write(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error writing cluster configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *clusterConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data clusterConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	class := data.Class.ValueString()
	if class == "" {
		// An imported resource only carries the ID.
		class = data.ID.ValueString()
		data.Class = types.StringValue(class)
	}

	raw, err := r.client.WithContext(ctx).GetClusterConfig(class)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Graylog answers 204 for a known class with no stored document
			// and 404 for a class it cannot resolve; either way there is
			// nothing left to manage.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cluster configuration", err.Error())
		return
	}

	serverDoc, err := refreshClusterConfigDocument(string(raw), data.ConfigJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster configuration", err.Error())
		return
	}
	stateDoc, err := CanonicalizeJSONFromString(data.ConfigJSON.ValueString())
	if err != nil {
		// Unparseable state cannot be compared; adopt the server document so
		// the next plan shows a diff instead of failing the refresh.
		stateDoc = ""
	}
	if serverDoc != stateDoc {
		// Drift in a managed key, or an import with no document yet. Storing
		// the projected server document makes the change visible in the next
		// plan; when the documents match, the practitioner's original
		// formatting is left untouched.
		data.ConfigJSON = types.StringValue(serverDoc)
	}

	data.ID = types.StringValue(class)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *clusterConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data clusterConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := data.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	if err := r.write(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Error writing cluster configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *clusterConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data clusterConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := data.Timeouts.Delete(ctx, 3*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Removing the document makes Graylog fall back to the default compiled
	// into the server; there is no way to "unset" a class any further.
	if err := r.client.WithContext(ctx).DeleteClusterConfig(data.Class.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting cluster configuration", err.Error())
	}
}

func (r *clusterConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("class"), req.ID)...)
}
