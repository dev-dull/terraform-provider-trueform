package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/trueform/terraform-provider-trueform/internal/client"
)

var (
	_ resource.Resource = &ServiceNFSResource{}
)

func NewServiceNFSResource() resource.Resource {
	return &ServiceNFSResource{}
}

type ServiceNFSResource struct {
	client *client.Client
}

type ServiceNFSResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Servers         types.Int64  `tfsdk:"servers"`
	UDPEnabled      types.Bool   `tfsdk:"udp_enabled"`
	V4              types.Bool   `tfsdk:"v4"`
	V4V3Owner       types.Bool   `tfsdk:"v4_v3owner"`
	V4Krb           types.Bool   `tfsdk:"v4_krb"`
	Bindip          types.List   `tfsdk:"bindip"`
	MountdPort      types.Int64  `tfsdk:"mountd_port"`
	RpclockdPort    types.Int64  `tfsdk:"rpcstatd_port"`
	AllowNonroot    types.Bool   `tfsdk:"allow_nonroot"`
	ManagedNFSv4ACL types.Bool   `tfsdk:"managed_nfsv4_acl"`
}

func (r *ServiceNFSResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_nfs"
}

func (r *ServiceNFSResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the NFS service configuration on TrueNAS. This resource must be configured and enabled for NFS shares to be accessible.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier (always 'nfs').",
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the NFS service is enabled and running.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"servers": schema.Int64Attribute{
				Description: "Number of NFS server instances to run. Recommended to match number of CPU cores.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(4),
			},
			"udp_enabled": schema.BoolAttribute{
				Description: "Enable UDP transport for NFS (NFSv3 only).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"v4": schema.BoolAttribute{
				Description: "Enable NFSv4 protocol.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"v4_v3owner": schema.BoolAttribute{
				Description: "Use NFSv3 ownership model for NFSv4.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"v4_krb": schema.BoolAttribute{
				Description: "Enable Kerberos for NFSv4.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"bindip": schema.ListAttribute{
				Description: "IP addresses to bind NFS service to. Empty list means all interfaces.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"mountd_port": schema.Int64Attribute{
				Description: "Port for mountd service. Set to 0 for random port.",
				Optional:    true,
				Computed:    true,
			},
			"rpcstatd_port": schema.Int64Attribute{
				Description: "Port for rpcstatd service. Set to 0 for random port.",
				Optional:    true,
				Computed:    true,
			},
			"allow_nonroot": schema.BoolAttribute{
				Description: "Allow non-root mount requests.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"managed_nfsv4_acl": schema.BoolAttribute{
				Description: "Enable managed NFSv4 ACL support.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *ServiceNFSResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ServiceNFSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceNFSResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Configuring NFS service")

	// Update NFS configuration
	configData := r.buildConfigData(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var configResult map[string]interface{}
	err := r.client.Call(ctx, "nfs.update", []interface{}{configData}, &configResult)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Configuring NFS Service",
			"Could not configure NFS service: "+err.Error(),
		)
		return
	}

	// Start or stop service based on enabled flag
	if plan.Enabled.ValueBool() {
		err = r.client.Call(ctx, "service.start", []interface{}{"nfs", map[string]interface{}{"silent": false}}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Starting NFS Service",
				"Could not start NFS service: "+err.Error(),
			)
			return
		}
	}

	// Read back the configuration
	if err := r.readService(ctx, &plan); err != nil {
		resp.Diagnostics.AddError(
			"Error Reading NFS Service",
			"Could not read NFS service after configuration: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue("nfs")
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceNFSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServiceNFSResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readService(ctx, &state); err != nil {
		resp.Diagnostics.AddError(
			"Error Reading NFS Service",
			"Could not read NFS service: "+err.Error(),
		)
		return
	}

	state.ID = types.StringValue("nfs")
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceNFSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceNFSResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ServiceNFSResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating NFS service configuration")

	// Update NFS configuration
	configData := r.buildConfigData(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var configResult map[string]interface{}
	err := r.client.Call(ctx, "nfs.update", []interface{}{configData}, &configResult)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating NFS Service",
			"Could not update NFS service: "+err.Error(),
		)
		return
	}

	// Handle service state change
	if plan.Enabled.ValueBool() && !state.Enabled.ValueBool() {
		err = r.client.Call(ctx, "service.start", []interface{}{"nfs", map[string]interface{}{"silent": false}}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Starting NFS Service",
				"Could not start NFS service: "+err.Error(),
			)
			return
		}
	} else if !plan.Enabled.ValueBool() && state.Enabled.ValueBool() {
		err = r.client.Call(ctx, "service.stop", []interface{}{"nfs", map[string]interface{}{"silent": false}}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Stopping NFS Service",
				"Could not stop NFS service: "+err.Error(),
			)
			return
		}
	} else if plan.Enabled.ValueBool() {
		// Restart service to apply config changes
		err = r.client.Call(ctx, "service.restart", []interface{}{"nfs", map[string]interface{}{"silent": false}}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Restarting NFS Service",
				"Could not restart NFS service: "+err.Error(),
			)
			return
		}
	}

	// Read back the configuration
	if err := r.readService(ctx, &plan); err != nil {
		resp.Diagnostics.AddError(
			"Error Reading NFS Service",
			"Could not read NFS service after update: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue("nfs")
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceNFSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceNFSResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Disabling NFS service")

	// Stop the service
	err := r.client.Call(ctx, "service.stop", []interface{}{"nfs", map[string]interface{}{"silent": false}}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Stopping NFS Service",
			"Could not stop NFS service: "+err.Error(),
		)
		return
	}
}

func (r *ServiceNFSResource) buildConfigData(ctx context.Context, plan *ServiceNFSResourceModel, diagnostics *diag.Diagnostics) map[string]interface{} {
	configData := map[string]interface{}{}

	if !plan.Servers.IsNull() && !plan.Servers.IsUnknown() {
		configData["servers"] = plan.Servers.ValueInt64()
	}
	if !plan.UDPEnabled.IsNull() && !plan.UDPEnabled.IsUnknown() {
		configData["udp"] = plan.UDPEnabled.ValueBool()
	}
	if !plan.V4.IsNull() && !plan.V4.IsUnknown() {
		configData["v4"] = plan.V4.ValueBool()
	}
	if !plan.V4V3Owner.IsNull() && !plan.V4V3Owner.IsUnknown() {
		configData["v4_v3owner"] = plan.V4V3Owner.ValueBool()
	}
	if !plan.V4Krb.IsNull() && !plan.V4Krb.IsUnknown() {
		configData["v4_krb"] = plan.V4Krb.ValueBool()
	}
	if !plan.Bindip.IsNull() && !plan.Bindip.IsUnknown() {
		var bindips []string
		diags := plan.Bindip.ElementsAs(ctx, &bindips, false)
		diagnostics.Append(diags...)
		if !diagnostics.HasError() {
			configData["bindip"] = bindips
		}
	}
	if !plan.MountdPort.IsNull() && !plan.MountdPort.IsUnknown() {
		configData["mountd_port"] = plan.MountdPort.ValueInt64()
	}
	if !plan.RpclockdPort.IsNull() && !plan.RpclockdPort.IsUnknown() {
		configData["rpcstatd_port"] = plan.RpclockdPort.ValueInt64()
	}
	if !plan.AllowNonroot.IsNull() && !plan.AllowNonroot.IsUnknown() {
		configData["allow_nonroot"] = plan.AllowNonroot.ValueBool()
	}
	if !plan.ManagedNFSv4ACL.IsNull() && !plan.ManagedNFSv4ACL.IsUnknown() {
		configData["managed_nfsv4_acl"] = plan.ManagedNFSv4ACL.ValueBool()
	}

	return configData
}

func (r *ServiceNFSResource) readService(ctx context.Context, model *ServiceNFSResourceModel) error {
	// Get NFS configuration
	var config map[string]interface{}
	err := r.client.Call(ctx, "nfs.config", []interface{}{}, &config)
	if err != nil {
		return fmt.Errorf("failed to get NFS config: %w", err)
	}

	// Get service state
	var serviceState map[string]interface{}
	err = r.client.Call(ctx, "service.query", []interface{}{
		[][]interface{}{{"service", "=", "nfs"}},
	}, &serviceState)
	if err != nil {
		return fmt.Errorf("failed to get service state: %w", err)
	}

	// Check if service is running
	var services []map[string]interface{}
	err = r.client.Call(ctx, "service.query", []interface{}{
		[][]interface{}{{"service", "=", "nfs"}},
	}, &services)
	if err != nil {
		return fmt.Errorf("failed to query service: %w", err)
	}

	if len(services) > 0 {
		if state, ok := services[0]["state"].(string); ok {
			model.Enabled = types.BoolValue(state == "RUNNING")
		}
	}

	// Map config values
	if servers, ok := config["servers"].(float64); ok {
		model.Servers = types.Int64Value(int64(servers))
	}
	if udp, ok := config["udp"].(bool); ok {
		model.UDPEnabled = types.BoolValue(udp)
	}
	if v4, ok := config["v4"].(bool); ok {
		model.V4 = types.BoolValue(v4)
	}
	if v4v3owner, ok := config["v4_v3owner"].(bool); ok {
		model.V4V3Owner = types.BoolValue(v4v3owner)
	}
	if v4krb, ok := config["v4_krb"].(bool); ok {
		model.V4Krb = types.BoolValue(v4krb)
	}
	if bindip, ok := config["bindip"].([]interface{}); ok {
		ips := make([]string, len(bindip))
		for i, ip := range bindip {
			ips[i] = ip.(string)
		}
		ipValues, diags := types.ListValueFrom(ctx, types.StringType, ips)
		if !diags.HasError() {
			model.Bindip = ipValues
		}
	}
	if mountdPort, ok := config["mountd_port"].(float64); ok {
		model.MountdPort = types.Int64Value(int64(mountdPort))
	}
	if rpcstatdPort, ok := config["rpcstatd_port"].(float64); ok {
		model.RpclockdPort = types.Int64Value(int64(rpcstatdPort))
	}
	if allowNonroot, ok := config["allow_nonroot"].(bool); ok {
		model.AllowNonroot = types.BoolValue(allowNonroot)
	}
	if managedNfsv4Acl, ok := config["managed_nfsv4_acl"].(bool); ok {
		model.ManagedNFSv4ACL = types.BoolValue(managedNfsv4Acl)
	}

	return nil
}
