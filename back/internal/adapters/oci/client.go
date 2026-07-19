package oci

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type Client struct {
	provider common.ConfigurationProvider
	tenancy  string
}

type Snapshot struct {
	CapturedAt          time.Time            `json:"captured_at"`
	HomeRegion          string               `json:"home_region"`
	Regions             []Region             `json:"regions"`
	AvailabilityDomains []AvailabilityDomain `json:"availability_domains"`
	Compartments        []Compartment        `json:"compartments"`
	Instances           []Instance           `json:"instances"`
	VCNs                []VCN                `json:"vcns"`
	Subnets             []Subnet             `json:"subnets"`
	AutonomousDatabases []AutonomousDatabase `json:"autonomous_databases"`
	DBSystems           []DBSystem           `json:"db_systems"`
}

type Region struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Home   bool   `json:"home"`
}

type AvailabilityDomain struct {
	Name string `json:"name"`
}

type Compartment struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	LifecycleState string `json:"lifecycle_state"`
	ParentID       string `json:"parent_id,omitempty"`
}

type Instance struct {
	ID                 string            `json:"id"`
	DisplayName        string            `json:"display_name"`
	LifecycleState     string            `json:"lifecycle_state"`
	Shape              string            `json:"shape"`
	AvailabilityDomain string            `json:"availability_domain"`
	FaultDomain        string            `json:"fault_domain"`
	Region             string            `json:"region"`
	CompartmentID      string            `json:"compartment_id"`
	OCPUs              float32           `json:"ocpus"`
	MemoryGB           float32           `json:"memory_gb"`
	TimeCreated        *common.SDKTime   `json:"time_created,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
}

type VCN struct {
	ID             string          `json:"id"`
	DisplayName    string          `json:"display_name"`
	CIDRBlocks     []string        `json:"cidr_blocks"`
	LifecycleState string          `json:"lifecycle_state"`
	DNSLabel       string          `json:"dns_label"`
	CompartmentID  string          `json:"compartment_id"`
	TimeCreated    *common.SDKTime `json:"time_created,omitempty"`
}

type Subnet struct {
	ID                     string `json:"id"`
	DisplayName            string `json:"display_name"`
	CIDRBlock              string `json:"cidr_block"`
	AvailabilityDomain     string `json:"availability_domain,omitempty"`
	LifecycleState         string `json:"lifecycle_state"`
	VCNID                  string `json:"vcn_id"`
	CompartmentID          string `json:"compartment_id"`
	ProhibitPublicIPOnVNIC bool   `json:"prohibit_public_ip_on_vnic"`
}

type AutonomousDatabase struct {
	ID               string          `json:"id"`
	DisplayName      string          `json:"display_name"`
	DBName           string          `json:"db_name"`
	LifecycleState   string          `json:"lifecycle_state"`
	LifecycleDetails string          `json:"lifecycle_details,omitempty"`
	CompartmentID    string          `json:"compartment_id"`
	Workload         string          `json:"workload"`
	ComputeModel     string          `json:"compute_model"`
	ComputeCount     float32         `json:"compute_count"`
	StorageTB        int             `json:"storage_tb"`
	FreeTier         bool            `json:"free_tier"`
	TimeCreated      *common.SDKTime `json:"time_created,omitempty"`
}

type DBSystem struct {
	ID                 string          `json:"id"`
	DisplayName        string          `json:"display_name"`
	LifecycleState     string          `json:"lifecycle_state"`
	AvailabilityDomain string          `json:"availability_domain"`
	Shape              string          `json:"shape"`
	CompartmentID      string          `json:"compartment_id"`
	SubnetID           string          `json:"subnet_id"`
	DatabaseEdition    string          `json:"database_edition"`
	CPUCoreCount       int             `json:"cpu_core_count"`
	MemoryGB           int             `json:"memory_gb"`
	TimeCreated        *common.SDKTime `json:"time_created,omitempty"`
}

type LaunchInstanceInput struct {
	CompartmentID      string  `json:"compartment_id"`
	AvailabilityDomain string  `json:"availability_domain"`
	DisplayName        string  `json:"display_name"`
	Shape              string  `json:"shape"`
	ImageID            string  `json:"image_id"`
	SubnetID           string  `json:"subnet_id"`
	OCPUs              float32 `json:"ocpus"`
	MemoryGB           float32 `json:"memory_gb"`
	AssignPublicIP     bool    `json:"assign_public_ip"`
	SSHAuthorizedKey   string  `json:"ssh_authorized_key"`
}

type CreateAutonomousDatabaseInput struct {
	CompartmentID string  `json:"compartment_id"`
	DisplayName   string  `json:"display_name"`
	DBName        string  `json:"db_name"`
	AdminPassword string  `json:"admin_password"`
	Workload      string  `json:"workload"`
	ComputeCount  float32 `json:"compute_count"`
	StorageTB     int     `json:"storage_tb"`
	FreeTier      bool    `json:"free_tier"`
	AutoScaling   bool    `json:"auto_scaling"`
}

func New(configPath, profile string) (*Client, error) {
	if strings.TrimSpace(configPath) == "" {
		return nil, fmt.Errorf("OCI config path is required")
	}
	if strings.TrimSpace(profile) == "" {
		profile = "DEFAULT"
	}
	provider, err := common.ConfigurationProviderFromFileWithProfile(configPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("load OCI configuration: %w", err)
	}
	tenancy, err := provider.TenancyOCID()
	if err != nil {
		return nil, fmt.Errorf("load OCI tenancy: %w", err)
	}
	return &Client{provider: provider, tenancy: tenancy}, nil
}

func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(c.provider)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create OCI identity client: %w", err)
	}
	computeClient, err := core.NewComputeClientWithConfigurationProvider(c.provider)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create OCI compute client: %w", err)
	}
	networkClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(c.provider)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create OCI network client: %w", err)
	}
	databaseClient, err := database.NewDatabaseClientWithConfigurationProvider(c.provider)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create OCI database client: %w", err)
	}

	result := Snapshot{CapturedAt: time.Now().UTC()}
	regions, err := identityClient.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{TenancyId: common.String(c.tenancy)})
	if err != nil {
		return Snapshot{}, fmt.Errorf("list OCI regions: %w", err)
	}
	for _, item := range regions.Items {
		region := Region{Name: stringValue(item.RegionName), Status: string(item.Status), Home: boolValue(item.IsHomeRegion)}
		result.Regions = append(result.Regions, region)
		if region.Home {
			result.HomeRegion = region.Name
		}
	}
	domains, err := identityClient.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{CompartmentId: common.String(c.tenancy)})
	if err != nil {
		return Snapshot{}, fmt.Errorf("list OCI availability domains: %w", err)
	}
	for _, item := range domains.Items {
		result.AvailabilityDomains = append(result.AvailabilityDomains, AvailabilityDomain{Name: stringValue(item.Name)})
	}

	result.Compartments = append(result.Compartments, Compartment{ID: c.tenancy, Name: "root tenancy", LifecycleState: "ACTIVE"})
	compartments, err := identityClient.ListCompartments(ctx, identity.ListCompartmentsRequest{
		CompartmentId:          common.String(c.tenancy),
		CompartmentIdInSubtree: common.Bool(true),
		AccessLevel:            identity.ListCompartmentsAccessLevelAny,
		LifecycleState:         identity.CompartmentLifecycleStateActive,
		Limit:                  common.Int(1000),
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("list OCI compartments: %w", err)
	}
	for _, item := range compartments.Items {
		result.Compartments = append(result.Compartments, Compartment{ID: stringValue(item.Id), Name: stringValue(item.Name), Description: stringValue(item.Description), LifecycleState: string(item.LifecycleState), ParentID: stringValue(item.CompartmentId)})
	}

	type resources struct {
		instances           []Instance
		vcns                []VCN
		subnets             []Subnet
		autonomousDatabases []AutonomousDatabase
		dbSystems           []DBSystem
		err                 error
	}
	resourceCh := make(chan resources, len(result.Compartments))
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for _, compartment := range result.Compartments {
		compartmentID := compartment.ID
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			bundle := resources{}
			instances, requestErr := computeClient.ListInstances(ctx, core.ListInstancesRequest{CompartmentId: common.String(compartmentID), Limit: common.Int(1000)})
			if requestErr != nil {
				bundle.err = fmt.Errorf("list instances in compartment %s: %w", compartmentID, requestErr)
				resourceCh <- bundle
				return
			}
			for _, item := range instances.Items {
				if item.LifecycleState == core.InstanceLifecycleStateTerminated {
					continue
				}
				mapped := Instance{ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), LifecycleState: string(item.LifecycleState), Shape: stringValue(item.Shape), AvailabilityDomain: stringValue(item.AvailabilityDomain), FaultDomain: stringValue(item.FaultDomain), Region: stringValue(item.Region), CompartmentID: stringValue(item.CompartmentId), TimeCreated: item.TimeCreated, Tags: item.FreeformTags}
				if item.ShapeConfig != nil {
					mapped.OCPUs = float32Value(item.ShapeConfig.Ocpus)
					mapped.MemoryGB = float32Value(item.ShapeConfig.MemoryInGBs)
				}
				bundle.instances = append(bundle.instances, mapped)
			}
			vcns, requestErr := networkClient.ListVcns(ctx, core.ListVcnsRequest{CompartmentId: common.String(compartmentID), Limit: common.Int(1000)})
			if requestErr != nil {
				bundle.err = fmt.Errorf("list VCNs in compartment %s: %w", compartmentID, requestErr)
				resourceCh <- bundle
				return
			}
			for _, item := range vcns.Items {
				if item.LifecycleState == core.VcnLifecycleStateTerminated {
					continue
				}
				bundle.vcns = append(bundle.vcns, VCN{ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), CIDRBlocks: item.CidrBlocks, LifecycleState: string(item.LifecycleState), DNSLabel: stringValue(item.DnsLabel), CompartmentID: stringValue(item.CompartmentId), TimeCreated: item.TimeCreated})
			}
			subnets, requestErr := networkClient.ListSubnets(ctx, core.ListSubnetsRequest{CompartmentId: common.String(compartmentID), Limit: common.Int(1000)})
			if requestErr != nil {
				bundle.err = fmt.Errorf("list subnets in compartment %s: %w", compartmentID, requestErr)
				resourceCh <- bundle
				return
			}
			for _, item := range subnets.Items {
				if item.LifecycleState == core.SubnetLifecycleStateTerminated {
					continue
				}
				bundle.subnets = append(bundle.subnets, Subnet{ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), CIDRBlock: stringValue(item.CidrBlock), AvailabilityDomain: stringValue(item.AvailabilityDomain), LifecycleState: string(item.LifecycleState), VCNID: stringValue(item.VcnId), CompartmentID: stringValue(item.CompartmentId), ProhibitPublicIPOnVNIC: boolValue(item.ProhibitPublicIpOnVnic)})
			}
			autonomous, requestErr := databaseClient.ListAutonomousDatabases(ctx, database.ListAutonomousDatabasesRequest{CompartmentId: common.String(compartmentID), Limit: common.Int(1000)})
			if requestErr != nil {
				bundle.err = fmt.Errorf("list autonomous databases in compartment %s: %w", compartmentID, requestErr)
				resourceCh <- bundle
				return
			}
			for _, item := range autonomous.Items {
				bundle.autonomousDatabases = append(bundle.autonomousDatabases, AutonomousDatabase{ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), DBName: stringValue(item.DbName), LifecycleState: string(item.LifecycleState), LifecycleDetails: stringValue(item.LifecycleDetails), CompartmentID: stringValue(item.CompartmentId), Workload: string(item.DbWorkload), ComputeModel: string(item.ComputeModel), ComputeCount: float32Value(item.ComputeCount), StorageTB: intValue(item.DataStorageSizeInTBs), FreeTier: boolValue(item.IsFreeTier), TimeCreated: item.TimeCreated})
			}
			dbSystems, requestErr := databaseClient.ListDbSystems(ctx, database.ListDbSystemsRequest{CompartmentId: common.String(compartmentID), Limit: common.Int(1000)})
			if requestErr != nil {
				bundle.err = fmt.Errorf("list DB systems in compartment %s: %w", compartmentID, requestErr)
				resourceCh <- bundle
				return
			}
			for _, item := range dbSystems.Items {
				bundle.dbSystems = append(bundle.dbSystems, DBSystem{ID: stringValue(item.Id), DisplayName: stringValue(item.DisplayName), LifecycleState: string(item.LifecycleState), AvailabilityDomain: stringValue(item.AvailabilityDomain), Shape: stringValue(item.Shape), CompartmentID: stringValue(item.CompartmentId), SubnetID: stringValue(item.SubnetId), DatabaseEdition: string(item.DatabaseEdition), CPUCoreCount: intValue(item.CpuCoreCount), MemoryGB: intValue(item.MemorySizeInGBs), TimeCreated: item.TimeCreated})
			}
			resourceCh <- bundle
		}()
	}
	wg.Wait()
	close(resourceCh)
	for bundle := range resourceCh {
		if bundle.err != nil {
			return Snapshot{}, bundle.err
		}
		result.Instances = append(result.Instances, bundle.instances...)
		result.VCNs = append(result.VCNs, bundle.vcns...)
		result.Subnets = append(result.Subnets, bundle.subnets...)
		result.AutonomousDatabases = append(result.AutonomousDatabases, bundle.autonomousDatabases...)
		result.DBSystems = append(result.DBSystems, bundle.dbSystems...)
	}
	sort.Slice(result.Instances, func(i, j int) bool { return result.Instances[i].DisplayName < result.Instances[j].DisplayName })
	sort.Slice(result.VCNs, func(i, j int) bool { return result.VCNs[i].DisplayName < result.VCNs[j].DisplayName })
	sort.Slice(result.Subnets, func(i, j int) bool { return result.Subnets[i].DisplayName < result.Subnets[j].DisplayName })
	sort.Slice(result.AutonomousDatabases, func(i, j int) bool {
		return result.AutonomousDatabases[i].DisplayName < result.AutonomousDatabases[j].DisplayName
	})
	sort.Slice(result.DBSystems, func(i, j int) bool { return result.DBSystems[i].DisplayName < result.DBSystems[j].DisplayName })
	return result, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func float32Value(value *float32) float32 {
	if value == nil {
		return 0
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func (c *Client) LaunchInstance(ctx context.Context, input LaunchInstanceInput) (string, error) {
	if !strings.HasPrefix(input.CompartmentID, "ocid1.compartment.") && input.CompartmentID != c.tenancy || !strings.HasPrefix(input.ImageID, "ocid1.image.") || !strings.HasPrefix(input.SubnetID, "ocid1.subnet.") || strings.TrimSpace(input.AvailabilityDomain) == "" || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.Shape) == "" {
		return "", fmt.Errorf("invalid OCI instance launch input")
	}
	client, err := core.NewComputeClientWithConfigurationProvider(c.provider)
	if err != nil {
		return "", fmt.Errorf("create OCI compute client: %w", err)
	}
	metadata := map[string]string{}
	if strings.TrimSpace(input.SSHAuthorizedKey) != "" {
		metadata["ssh_authorized_keys"] = strings.TrimSpace(input.SSHAuthorizedKey)
	}
	details := core.LaunchInstanceDetails{AvailabilityDomain: common.String(input.AvailabilityDomain), CompartmentId: common.String(input.CompartmentID), DisplayName: common.String(input.DisplayName), Shape: common.String(input.Shape), SourceDetails: core.InstanceSourceViaImageDetails{ImageId: common.String(input.ImageID)}, CreateVnicDetails: &core.CreateVnicDetails{SubnetId: common.String(input.SubnetID), AssignPublicIp: common.Bool(input.AssignPublicIP)}, Metadata: metadata}
	if input.OCPUs > 0 || input.MemoryGB > 0 {
		details.ShapeConfig = &core.LaunchInstanceShapeConfigDetails{Ocpus: common.Float32(input.OCPUs), MemoryInGBs: common.Float32(input.MemoryGB)}
	}
	response, err := client.LaunchInstance(ctx, core.LaunchInstanceRequest{LaunchInstanceDetails: details})
	if err != nil {
		return "", fmt.Errorf("launch OCI instance: %w", err)
	}
	return stringValue(response.Instance.Id), nil
}

func (c *Client) CreateAutonomousDatabase(ctx context.Context, input CreateAutonomousDatabaseInput) (string, error) {
	if (!strings.HasPrefix(input.CompartmentID, "ocid1.compartment.") && input.CompartmentID != c.tenancy) || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.DBName) == "" || len(input.AdminPassword) < 12 {
		return "", fmt.Errorf("invalid Autonomous Database input")
	}
	workloads := map[string]database.CreateAutonomousDatabaseBaseDbWorkloadEnum{"OLTP": database.CreateAutonomousDatabaseBaseDbWorkloadOltp, "DW": database.CreateAutonomousDatabaseBaseDbWorkloadDw, "AJD": database.CreateAutonomousDatabaseBaseDbWorkloadAjd, "APEX": database.CreateAutonomousDatabaseBaseDbWorkloadApex}
	workload, ok := workloads[strings.ToUpper(input.Workload)]
	if !ok {
		return "", fmt.Errorf("unsupported Autonomous Database workload")
	}
	client, err := database.NewDatabaseClientWithConfigurationProvider(c.provider)
	if err != nil {
		return "", err
	}
	details := database.CreateAutonomousDatabaseDetails{CompartmentId: common.String(input.CompartmentID), DisplayName: common.String(input.DisplayName), DbName: common.String(input.DBName), AdminPassword: common.String(input.AdminPassword), DbWorkload: workload, IsFreeTier: common.Bool(input.FreeTier), IsAutoScalingEnabled: common.Bool(input.AutoScaling), LicenseModel: database.CreateAutonomousDatabaseBaseLicenseModelLicenseIncluded}
	if input.ComputeCount > 0 {
		details.ComputeCount = common.Float32(input.ComputeCount)
		details.ComputeModel = database.CreateAutonomousDatabaseBaseComputeModelEcpu
	}
	if input.StorageTB > 0 {
		details.DataStorageSizeInTBs = common.Int(input.StorageTB)
	}
	response, err := client.CreateAutonomousDatabase(ctx, database.CreateAutonomousDatabaseRequest{CreateAutonomousDatabaseDetails: details})
	if err != nil {
		return "", fmt.Errorf("create Autonomous Database: %w", err)
	}
	return stringValue(response.AutonomousDatabase.Id), nil
}

func (c *Client) InstanceAction(ctx context.Context, instanceID, action string) error {
	if !strings.HasPrefix(instanceID, "ocid1.instance.") {
		return fmt.Errorf("invalid OCI instance OCID")
	}
	actions := map[string]core.InstanceActionActionEnum{
		"start":    core.InstanceActionActionStart,
		"stop":     core.InstanceActionActionStop,
		"shutdown": core.InstanceActionActionSoftstop,
		"reboot":   core.InstanceActionActionSoftreset,
		"reset":    core.InstanceActionActionReset,
	}
	mapped, ok := actions[strings.ToLower(action)]
	if !ok {
		return fmt.Errorf("unsupported OCI instance action")
	}
	client, err := core.NewComputeClientWithConfigurationProvider(c.provider)
	if err != nil {
		return fmt.Errorf("create OCI compute client: %w", err)
	}
	_, err = client.InstanceAction(ctx, core.InstanceActionRequest{InstanceId: common.String(instanceID), Action: mapped})
	if err != nil {
		return fmt.Errorf("OCI instance action: %w", err)
	}
	return nil
}
