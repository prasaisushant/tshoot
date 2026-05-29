package models

// UIMode represents the current state of the UI
type UIMode string

const (
	ModeNormal  UIMode = "normal"
	ModeModal   UIMode = "modal"
	ModeFocused UIMode = "focused"
)

// ModalType represents which modal (if any) is currently open
type ModalType string

const (
	ModalNone        ModalType = ""
	ModalRefreshRate ModalType = "refresh_rate"
	ModalDocker      ModalType = "docker"
	ModalPing        ModalType = "ping"
	ModalFocus       ModalType = "focus_grid"
)

// AppState holds the global application state
type AppState struct {
	// UI Mode
	Mode        UIMode
	ActiveModal ModalType

	// Window dimensions
	Width  int
	Height int

	// Focus state (which panel is focused)
	FocusedPanel int

	// System metrics (Phase 2+)
	Metrics SystemMetrics

	// Process metrics (Phase 3+)
	TopCPUProcesses []ProcessStat
	TopMemProcesses []ProcessStat
	ProcessError    string

	// Network metrics (Phase 4+)
	OpenPorts    []PortStat
	IPRouteLines []string
	NetworkError string

	// Storage metrics (Phase 5+)
	StorageMounts        []StorageMount
	StorageDeviceEntries []StorageDeviceEntry
	StorageLoopCount     int
	StorageIOReadKB      float64
	StorageIOWriteKB     float64
	StorageListView      bool
	StorageError         string

	// Ping metrics (Phase 4+)
	PingResults []PingStat
	PingError   string

	// Docker metrics (Phase 5+)
	DockerContainers    []DockerContainerStat
	SelectedContainerID string
	SelectedContainer   string
	DockerModalIndex    int
	PingModalIndex      int
	// Inline edit state for ping modal
	PingEditing   bool
	PingEditIndex int
	PingEditLabel string
	PingEditHost  string
	PingEditField int // 0=label,1=host
	ContainerLogs []string
	DockerError   string
}

// SystemMetrics contains live values shown in dashboard panels.
type SystemMetrics struct {
	CPUPercent       float64
	MemoryPercent    float64
	SwapPercent      float64
	MemoryUsedMB     int
	MemoryTotalMB    int
	SwapUsedMB       int
	SwapTotalMB      int
	Load1            float64
	Load5            float64
	Load15           float64
	UptimeSeconds    float64
	Clock            string
	LastErrorSummary string
}

// ProcessStat holds process info for top lists.
type ProcessStat struct {
	PID        int
	Name       string
	CPUPercent float64
	MemoryMB   int
}

// PortStat represents an open socket mapped to a process when possible.
type PortStat struct {
	Port    int
	Proto   string
	PID     int
	Process string
}

// StorageMount represents one filesystem mount line.
type StorageMount struct {
	Mount  string
	FSType string
	UsePct string
	Size   string
}

// StorageDeviceEntry represents one block device entry line.
type StorageDeviceEntry struct {
	Name  string
	Size  string
	Type  string
	Mount string
}

// PingStat represents status/latency for one target.
type PingStat struct {
	Label     string
	Host      string
	Up        bool
	LatencyMS float64
}

// DockerContainerStat represents one docker container row.
type DockerContainerStat struct {
	ID     string
	Name   string
	Status string
	Ports  string
}

// NewAppState creates a new app state with sensible defaults
func NewAppState(width, height int) *AppState {
	return &AppState{
		Mode:         ModeNormal,
		ActiveModal:  ModalNone,
		Width:        width,
		Height:       height,
		FocusedPanel: 0,
		Metrics: SystemMetrics{
			Clock: "N/A",
		},
		TopCPUProcesses:      []ProcessStat{},
		TopMemProcesses:      []ProcessStat{},
		OpenPorts:            []PortStat{},
		IPRouteLines:         []string{},
		StorageMounts:        []StorageMount{},
		StorageDeviceEntries: []StorageDeviceEntry{},
		StorageLoopCount:     0,
		StorageIOReadKB:      0,
		StorageIOWriteKB:     0,
		StorageListView:      false,
		PingResults:          []PingStat{},
		DockerContainers:     []DockerContainerStat{},
		DockerModalIndex:     0,
		PingModalIndex:       0,
		PingEditing:          false,
		PingEditIndex:        -1,
		PingEditLabel:        "",
		PingEditHost:         "",
		PingEditField:        0,
		ContainerLogs:        []string{},
	}
}

// SetMode changes the UI mode and clears modal if leaving modal state
func (s *AppState) SetMode(mode UIMode) {
	s.Mode = mode
	if mode != ModeModal {
		s.ActiveModal = ModalNone
	}
}

// OpenModal switches to the given modal (single modal at a time)
func (s *AppState) OpenModal(modalType ModalType) {
	s.ActiveModal = modalType
	s.Mode = ModeModal
}

// CloseModal returns to normal mode
func (s *AppState) CloseModal() {
	s.ActiveModal = ModalNone
	s.Mode = ModeNormal
}
