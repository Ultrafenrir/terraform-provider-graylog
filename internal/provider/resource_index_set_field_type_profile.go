package provider

import (
	"context"
	"errors"
	"sort"

	"github.com/Ultrafenrir/terraform-provider-graylog/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type indexSetFieldTypeProfileResource struct{ client *client.Client }
type indexSetFieldTypeProfileModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	CustomFieldMappings types.Map    `tfsdk:"custom_field_mappings"`
}

func NewIndexSetFieldTypeProfileResource() resource.Resource {
	return &indexSetFieldTypeProfileResource{}
}

func (r *indexSetFieldTypeProfileResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "graylog_index_set_field_type_profile"
}

func (r *indexSetFieldTypeProfileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Graylog index set field type profile. Assign profiles with graylog_index_set.field_type_profile. Mapping changes apply to future indices; this resource does not rotate indices. Deleting a profile detaches it from every index set, including index sets outside Terraform.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, Description: "Profile name.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Profile description. Defaults to an empty string."},
			"custom_field_mappings": schema.MapAttribute{Required: true, ElementType: types.StringType, Description: "Field name to Graylog logical type mapping. An empty map clears all custom mappings. Supported types: string, string_fts, long, double, date, boolean, binary, geo-point, ip.", Validators: []validator.Map{
				mapvalidator.KeysAre(stringvalidator.LengthAtLeast(1)),
				mapvalidator.ValueStringsAre(stringvalidator.OneOf("string", "string_fts", "long", "double", "date", "boolean", "binary", "geo-point", "ip")),
			}},
		},
	}
}

func (r *indexSetFieldTypeProfileResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*client.Client)
	}
}

func profilePayload(ctx context.Context, data *indexSetFieldTypeProfileModel) (*client.IndexSetFieldTypeProfile, error) {
	mappings := map[string]string{}
	if diags := data.CustomFieldMappings.ElementsAs(ctx, &mappings, false); diags.HasError() {
		return nil, errors.New(diags.Errors()[0].Detail())
	}
	keys := make([]string, 0, len(mappings))
	for key := range mappings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	profile := &client.IndexSetFieldTypeProfile{Name: data.Name.ValueString(), Description: data.Description.ValueString(), CustomFieldMappings: make([]client.CustomFieldMapping, 0, len(keys))}
	for _, field := range keys {
		profile.CustomFieldMappings = append(profile.CustomFieldMappings, client.CustomFieldMapping{Field: field, Type: mappings[field]})
	}
	return profile, nil
}

func applyFieldTypeProfileReadState(ctx context.Context, data *indexSetFieldTypeProfileModel, profile *client.IndexSetFieldTypeProfile) {
	data.ID = types.StringValue(profile.ID)
	data.Name = types.StringValue(profile.Name)
	data.Description = types.StringValue(profile.Description)
	mappings := map[string]string{}
	for _, mapping := range profile.CustomFieldMappings {
		mappings[mapping.Field] = mapping.Type
	}
	data.CustomFieldMappings, _ = types.MapValueFrom(ctx, types.StringType, mappings)
}

func (r *indexSetFieldTypeProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data indexSetFieldTypeProfileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := profilePayload(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid field type profile", err.Error())
		return
	}
	profile, err := r.client.WithContext(ctx).CreateIndexSetFieldTypeProfile(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating field type profile", err.Error())
		return
	}
	applyFieldTypeProfileReadState(ctx, &data, profile)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetFieldTypeProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data indexSetFieldTypeProfileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profile, err := r.client.WithContext(ctx).GetIndexSetFieldTypeProfile(data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading field type profile", err.Error())
		return
	}
	applyFieldTypeProfileReadState(ctx, &data, profile)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetFieldTypeProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state indexSetFieldTypeProfileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := profilePayload(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid field type profile", err.Error())
		return
	}
	if err := r.client.WithContext(ctx).UpdateIndexSetFieldTypeProfile(state.ID.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("Error updating field type profile", err.Error())
		return
	}
	data.ID = state.ID
	// Persist the successful update before the read so a read failure retains its ID.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	profile, err := r.client.WithContext(ctx).GetIndexSetFieldTypeProfile(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading field type profile after update", err.Error())
		return
	}
	applyFieldTypeProfileReadState(ctx, &data, profile)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *indexSetFieldTypeProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data indexSetFieldTypeProfileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.WithContext(ctx).DeleteIndexSetFieldTypeProfile(data.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Error deleting field type profile", err.Error())
	}
}

func (r *indexSetFieldTypeProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
