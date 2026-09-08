package model

import "time"

const CatalogAPIVersion = "deployer.home-server-project.io/v1alpha1"

type PodmanCapabilities struct {
	Version                  string `json:"version"`
	LibpodAPIVersion         string `json:"libpodApiVersion"`
	MinLibpodAPIVersion      string `json:"minLibpodApiVersion,omitempty"`
	Architecture             string `json:"architecture,omitempty"`
	Rootless                 bool   `json:"rootless"`
	CgroupManager            string `json:"cgroupManager,omitempty"`
	CgroupVersion            string `json:"cgroupVersion,omitempty"`
	NetworkBackend           string `json:"networkBackend,omitempty"`
	QuadletList              bool   `json:"quadletList"`
	QuadletPrint             bool   `json:"quadletPrint"`
	QuadletInstall           bool   `json:"quadletInstall"`
	QuadletRemove            bool   `json:"quadletRemove"`
	NativeApplicationInstall bool   `json:"nativeApplicationInstall"`
}

type HostCapabilities struct {
	Podman        PodmanCapabilities   `json:"podman"`
	Systemd       SystemdCapabilities  `json:"systemd"`
	Security      SecurityCapabilities `json:"security"`
	ApprovedRoots []ApprovedRootInfo   `json:"approvedRoots"`
}

type SystemdCapabilities struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
}

type SecurityCapabilities struct {
	Backend   string `json:"backend"`
	Enabled   bool   `json:"enabled"`
	Enforcing bool   `json:"enforcing"`
}

type ApprovedRootInfo struct {
	ID       string `json:"id"`
	HostPath string `json:"hostPath"`
}

type Quadlet struct {
	Name     string `json:"Name" yaml:"name"`
	UnitName string `json:"UnitName" yaml:"unitName"`
	Path     string `json:"Path" yaml:"path"`
	Status   string `json:"Status" yaml:"status"`
	App      string `json:"App" yaml:"app"`
}

type RuntimeContainer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Health      string            `json:"health,omitempty"`
	SystemdUnit string            `json:"systemdUnit,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type DiscoveredQuadlet struct {
	Quadlet  Quadlet           `json:"quadlet"`
	Runtime  *RuntimeContainer `json:"runtime,omitempty"`
	Managed  bool              `json:"managed"`
	Instance string            `json:"instance,omitempty"`
}

type Catalog struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   AppMetadata `yaml:"metadata" json:"metadata"`
	Spec       AppSpec     `yaml:"spec" json:"spec"`
}

type AppMetadata struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
}

type AppSpec struct {
	MinPodmanVersion string          `yaml:"minPodmanVersion" json:"minPodmanVersion"`
	Inputs           []InputSpec     `yaml:"inputs" json:"inputs"`
	Directories      []DirectorySpec `yaml:"directories" json:"directories"`
	Resources        []ResourceSpec  `yaml:"resources" json:"resources"`
	Update           UpdatePolicy    `yaml:"update" json:"update"`
	Health           HealthContract  `yaml:"health" json:"health"`
	Backup           BackupContract  `yaml:"backup" json:"backup"`
}

type InputSpec struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default" json:"default,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
}

type DirectorySpec struct {
	Name           string `yaml:"name" json:"name"`
	RootID         string `yaml:"rootId" json:"rootId"`
	SubpathInput   string `yaml:"subpathInput" json:"subpathInput"`
	ContainerPath  string `yaml:"containerPath" json:"containerPath"`
	ReadOnly       bool   `yaml:"readOnly" json:"readOnly"`
	SecurityIntent string `yaml:"securityIntent" json:"securityIntent"`
}

type ResourceSpec struct {
	Name     string `yaml:"name" json:"name"`
	Template string `yaml:"template" json:"template"`
	Start    bool   `yaml:"start" json:"start"`
}

type UpdatePolicy struct {
	Mode    string `yaml:"mode" json:"mode"`
	Channel string `yaml:"channel" json:"channel,omitempty"`
}

type HealthContract struct {
	Strategy        string `yaml:"strategy" json:"strategy"`
	PrimaryResource string `yaml:"primaryResource" json:"primaryResource,omitempty"`
	TimeoutSeconds  int    `yaml:"timeoutSeconds" json:"timeoutSeconds,omitempty"`
}

type BackupContract struct {
	Strategy    string   `yaml:"strategy" json:"strategy"`
	Consistency string   `yaml:"consistency" json:"consistency"`
	Includes    []string `yaml:"includes" json:"includes"`
	Provider    string   `yaml:"provider" json:"provider,omitempty"`
}

type Instance struct {
	ID             string                   `json:"id"`
	CatalogID      string                   `json:"catalogId"`
	CatalogVersion string                   `json:"catalogVersion"`
	Parameters     map[string]string        `json:"parameters"`
	Resources      map[string]ResourceState `json:"resources"`
	Directories    []ResolvedDirectory      `json:"directories"`
	InstalledAt    time.Time                `json:"installedAt"`
	UpdatedAt      time.Time                `json:"updatedAt"`
	RemovedAt      *time.Time               `json:"removedAt,omitempty"`
	History        []HistoryEntry           `json:"history"`
}

type ResourceState struct {
	Name     string `json:"name"`
	UnitName string `json:"unitName,omitempty"`
	SHA256   string `json:"sha256"`
	Source   string `json:"source"`
}

type InstanceStatus struct {
	Instance  Instance         `json:"instance"`
	Resources []ResourceStatus `json:"resources"`
}

type ResourceStatus struct {
	Name          string            `json:"name"`
	UnitName      string            `json:"unitName,omitempty"`
	QuadletStatus string            `json:"quadletStatus,omitempty"`
	SystemdStatus string            `json:"systemdStatus,omitempty"`
	Runtime       *RuntimeContainer `json:"runtime,omitempty"`
}

type ResolvedDirectory struct {
	Name           string `json:"name"`
	RootID         string `json:"rootId"`
	RelativePath   string `json:"relativePath"`
	HostPath       string `json:"hostPath"`
	AgentPath      string `json:"agentPath"`
	ContainerPath  string `json:"containerPath"`
	ReadOnly       bool   `json:"readOnly"`
	SecurityIntent string `json:"securityIntent"`
	MountSuffix    string `json:"mountSuffix"`
}

type HistoryEntry struct {
	At        time.Time `json:"at"`
	Operation string    `json:"operation"`
	Result    string    `json:"result"`
	Note      string    `json:"note,omitempty"`
}

type PlanRequest struct {
	Operation  string            `json:"operation"`
	AppID      string            `json:"appId"`
	InstanceID string            `json:"instanceId"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

type Plan struct {
	ID             string              `json:"id"`
	Digest         string              `json:"digest"`
	Operation      string              `json:"operation"`
	AppID          string              `json:"appId"`
	CatalogVersion string              `json:"catalogVersion"`
	InstanceID     string              `json:"instanceId"`
	Parameters     map[string]string   `json:"parameters,omitempty"`
	Directories    []ResolvedDirectory `json:"directories,omitempty"`
	Resources      []RenderedResource  `json:"resources,omitempty"`
	Drift          []DriftRecord       `json:"drift,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	ExpiresAt      time.Time           `json:"expiresAt"`
}

type RenderedResource struct {
	Name           string `json:"name"`
	UnitName       string `json:"unitName,omitempty"`
	Content        string `json:"content"`
	Start          bool   `json:"start"`
	SHA256         string `json:"sha256"`
	PreviousSHA256 string `json:"previousSha256,omitempty"`
}

type DriftRecord struct {
	Resource     string `json:"resource"`
	ExpectedHash string `json:"expectedHash"`
	ActualHash   string `json:"actualHash"`
}

type OperationResult struct {
	PlanID     string    `json:"planId"`
	InstanceID string    `json:"instanceId"`
	Operation  string    `json:"operation"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	At         time.Time `json:"at"`
}
