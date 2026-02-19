package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/trueform/terraform-provider-trueform/internal/client"
)

var (
	_ resource.Resource = &ServiceDockerResource{}
)

func NewServiceDockerResource() resource.Resource {
	return &ServiceDockerResource{}
}

type ServiceDockerResource struct {
	client *client.Client
}

type ServiceDockerResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Pool               types.String `tfsdk:"pool"`
	NvidiaEnabled      types.Bool   `tfsdk:"nvidia"`
	EnableImageUpdates types.Bool   `tfsdk:"enable_image_updates"`
	Status             types.String `tfsdk:"status"`
}

func (r *ServiceDockerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_docker"
}

func (r *ServiceDockerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the Docker/Apps service configuration on TrueNAS. A pool must be configured for Docker before applications can be deployed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier (always 'docker').",
				Computed:    true,
			},
			"pool": schema.StringAttribute{
				Description: "The storage pool to use for Docker/Apps data.",
				Required:    true,
			},
			"nvidia": schema.BoolAttribute{
				Description: "Enable NVIDIA GPU support for containers.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"enable_image_updates": schema.BoolAttribute{
				Description: "Automatically check for Docker image updates.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				Description: "Current Docker service status (e.g., RUNNING, INITIALIZING, STOPPED).",
				Computed:    true,
			},
		},
	}
}

func (r *ServiceDockerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServiceDockerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceDockerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Configuring Docker service", map[string]interface{}{
		"pool": plan.Pool.ValueString(),
	})

	if err := r.updateDocker(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error Configuring Docker Service", err.Error())
		return
	}

	if err := r.waitForRunning(ctx); err != nil {
		resp.Diagnostics.AddError("Error Waiting for Docker Service", err.Error())
		return
	}

	if err := r.readDocker(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error Reading Docker Service", "Could not read Docker service after configuration: "+err.Error())
		return
	}

	plan.ID = types.StringValue("docker")
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceDockerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServiceDockerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.readDocker(ctx, &state); err != nil {
		resp.Diagnostics.AddError("Error Reading Docker Service", "Could not read Docker service: "+err.Error())
		return
	}

	// If no pool is configured, remove from state
	if state.Pool.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue("docker")
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceDockerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceDockerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Docker service configuration", map[string]interface{}{
		"pool": plan.Pool.ValueString(),
	})

	if err := r.updateDocker(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error Updating Docker Service", err.Error())
		return
	}

	if err := r.waitForRunning(ctx); err != nil {
		resp.Diagnostics.AddError("Error Waiting for Docker Service", err.Error())
		return
	}

	if err := r.readDocker(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error Reading Docker Service", "Could not read Docker service after update: "+err.Error())
		return
	}

	plan.ID = types.StringValue("docker")
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ServiceDockerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Unconfiguring Docker service")

	// Unconfigure Docker by setting pool to null
	updateData := map[string]interface{}{
		"pool": nil,
	}

	var jobID float64
	err := r.client.Call(ctx, "docker.update", []interface{}{updateData}, &jobID)
	if err != nil {
		resp.Diagnostics.AddError("Error Unconfiguring Docker Service", "Could not unconfigure Docker service: "+err.Error())
		return
	}

	if _, err := r.client.WaitForJob(ctx, int64(jobID), 5*time.Minute); err != nil {
		resp.Diagnostics.AddError("Error Unconfiguring Docker Service", "Docker unconfigure job failed: "+err.Error())
		return
	}
}

func (r *ServiceDockerResource) updateDocker(ctx context.Context, plan *ServiceDockerResourceModel) error {
	updateData := map[string]interface{}{
		"pool": plan.Pool.ValueString(),
	}

	if !plan.NvidiaEnabled.IsNull() && !plan.NvidiaEnabled.IsUnknown() {
		updateData["nvidia"] = plan.NvidiaEnabled.ValueBool()
	}
	if !plan.EnableImageUpdates.IsNull() && !plan.EnableImageUpdates.IsUnknown() {
		updateData["enable_image_updates"] = plan.EnableImageUpdates.ValueBool()
	}

	var jobID float64
	err := r.client.Call(ctx, "docker.update", []interface{}{updateData}, &jobID)
	if err != nil {
		return fmt.Errorf("could not update Docker service: %w", err)
	}

	if _, err := r.client.WaitForJob(ctx, int64(jobID), 5*time.Minute); err != nil {
		return fmt.Errorf("Docker update job failed: %w", err)
	}

	return nil
}

func (r *ServiceDockerResource) waitForRunning(ctx context.Context) error {
	timeout := 5 * time.Minute
	pollInterval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for Docker service to become RUNNING after %v", timeout)
		}

		var status map[string]interface{}
		err := r.client.Call(ctx, "docker.status", []interface{}{}, &status)
		if err != nil {
			tflog.Debug(ctx, "Error polling Docker status, retrying", map[string]interface{}{
				"error": err.Error(),
			})
			time.Sleep(pollInterval)
			continue
		}

		state, _ := status["status"].(string)
		tflog.Debug(ctx, "Docker service status", map[string]interface{}{
			"status": state,
		})

		if state == "RUNNING" {
			return nil
		}

		time.Sleep(pollInterval)
	}
}

func (r *ServiceDockerResource) readDocker(ctx context.Context, model *ServiceDockerResourceModel) error {
	// Get Docker configuration
	var config map[string]interface{}
	err := r.client.Call(ctx, "docker.config", []interface{}{}, &config)
	if err != nil {
		return fmt.Errorf("failed to get Docker config: %w", err)
	}

	if pool, ok := config["pool"].(string); ok {
		model.Pool = types.StringValue(pool)
	} else {
		model.Pool = types.StringValue("")
	}
	if nvidia, ok := config["nvidia"].(bool); ok {
		model.NvidiaEnabled = types.BoolValue(nvidia)
	}
	if enableImageUpdates, ok := config["enable_image_updates"].(bool); ok {
		model.EnableImageUpdates = types.BoolValue(enableImageUpdates)
	}

	// Get Docker status
	var status map[string]interface{}
	err = r.client.Call(ctx, "docker.status", []interface{}{}, &status)
	if err != nil {
		return fmt.Errorf("failed to get Docker status: %w", err)
	}

	if state, ok := status["status"].(string); ok {
		model.Status = types.StringValue(state)
	}

	return nil
}
