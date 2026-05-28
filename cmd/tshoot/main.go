package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/prasaisushant/tshoot/internal/collectors"
	"github.com/prasaisushant/tshoot/internal/models"
	"github.com/prasaisushant/tshoot/internal/ui"
)

// App is the main application model
type App struct {
	state       *models.AppState
	config      *Config
	theme       *ui.Theme
	cpuCalc     *collectors.CPUCalculator
	procCalc    *collectors.ProcessCollector
	diskIO      *collectors.DiskIOCollector
	pingTargets []collectors.PingTarget
	docker      *collectors.DockerCollector
}

type metricsTickMsg time.Time

// NewApp creates a new application
func NewApp() *App {
	config := LoadConfig()
	state := models.NewAppState(80, 24) // Default size, will be updated on first render

	theme := ui.GetTheme(config.General.Theme)
	dockerCollector, _ := collectors.NewDockerCollector()

	// Try to load ping targets from config file, fall back to defaults
	pingTargets := collectors.DefaultPingTargets()
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".config", "tshoot", "ping_targets.toml")
		if targets, err := collectors.LoadPingTargetsFromFile(path); err == nil && len(targets) > 0 {
			pingTargets = make([]collectors.PingTarget, 0, len(targets))
			for _, t := range targets {
				pingTargets = append(pingTargets, collectors.PingTarget{Label: t.Label, Host: t.Host})
			}
		}
	}
	pingTargets = collectors.EnsureDefaultPingTargets(pingTargets)

	return &App{
		state:       state,
		config:      config,
		theme:       theme,
		cpuCalc:     &collectors.CPUCalculator{},
		procCalc:    collectors.NewProcessCollector(),
		pingTargets: pingTargets,
		diskIO:      collectors.NewDiskIOCollector(),
		docker:      dockerCollector,
	}
}

// Init initializes the app (bubbletea Model interface)
func (a *App) Init() tea.Cmd {
	return scheduleMetricsTick()
}

// Update handles messages (bubbletea Model interface)
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		a.state.Width = msg.Width
		a.state.Height = msg.Height
		return a, nil
	case metricsTickMsg:
		if !a.state.IsPaused {
			snapshot, err := collectors.CollectSnapshot(a.cpuCalc)
			a.state.Metrics.CPUPercent = snapshot.CPUPercent
			a.state.Metrics.MemoryPercent = snapshot.MemoryPercent
			a.state.Metrics.SwapPercent = snapshot.SwapPercent
			a.state.Metrics.MemoryUsedMB = snapshot.MemoryUsedMB
			a.state.Metrics.MemoryTotalMB = snapshot.MemoryTotalMB
			a.state.Metrics.SwapUsedMB = snapshot.SwapUsedMB
			a.state.Metrics.SwapTotalMB = snapshot.SwapTotalMB
			a.state.Metrics.Load1 = snapshot.Load1
			a.state.Metrics.Load5 = snapshot.Load5
			a.state.Metrics.Load15 = snapshot.Load15
			a.state.Metrics.UptimeSeconds = snapshot.UptimeSeconds
			a.state.Metrics.Clock = snapshot.Clock
			if err != nil {
				a.state.Metrics.LastErrorSummary = err.Error()
			} else {
				a.state.Metrics.LastErrorSummary = ""
			}

			topCPU, topMem, perr := collectors.CollectTopProcesses(a.procCalc, 3)
			if perr != nil {
				a.state.ProcessError = perr.Error()
			} else {
				a.state.ProcessError = ""
				a.state.TopCPUProcesses = convertCollectorProcesses(topCPU)
				a.state.TopMemProcesses = convertCollectorProcesses(topMem)
			}

			ports, nerr1 := collectors.CollectOpenPorts(8)
			ipRoutes, nerr2 := collectors.CollectIPRouteLines(6)
			a.state.OpenPorts = ports
			a.state.IPRouteLines = ipRoutes
			switch {
			case nerr1 != nil && nerr2 != nil:
				a.state.NetworkError = nerr1.Error() + "," + nerr2.Error()
			case nerr1 != nil:
				a.state.NetworkError = nerr1.Error()
			case nerr2 != nil:
				a.state.NetworkError = nerr2.Error()
			default:
				a.state.NetworkError = ""
			}

			pings, perr2 := collectors.CollectPingStatuses(a.pingTargets, 1200*time.Millisecond)
			a.state.PingResults = pings
			if perr2 != nil {
				a.state.PingError = perr2.Error()
			} else {
				a.state.PingError = ""
			}

			mounts, devices, loopCount, serr := collectors.CollectStorageSummary(4)
			a.state.StorageMounts = mounts
			a.state.StorageDeviceEntries = devices
			a.state.StorageLoopCount = loopCount
			if serr != nil {
				a.state.StorageError = serr.Error()
			} else {
				a.state.StorageError = ""
			}

			readKB, writeKB, ioErr := a.diskIO.CollectIOSpeeds()
			a.state.StorageIOReadKB = readKB
			a.state.StorageIOWriteKB = writeKB
			if ioErr != nil {
				if a.state.StorageError != "" {
					a.state.StorageError += "; " + ioErr.Error()
				} else {
					a.state.StorageError = ioErr.Error()
				}
			}

			a.collectDockerSnapshot()
		}
		return a, scheduleMetricsTick()
	case tea.QuitMsg:
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) collectDockerSnapshot() {
	if a.docker == nil {
		a.state.DockerError = "docker unavailable"
		a.state.DockerContainers = []models.DockerContainerStat{}
		a.state.ContainerLogs = []string{"Docker unavailable"}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	containers, err := a.docker.CollectContainers(ctx, 6)
	if err != nil {
		a.state.DockerError = err.Error()
		a.state.DockerContainers = []models.DockerContainerStat{}
		a.state.ContainerLogs = []string{"Docker error: " + err.Error()}
		return
	}
	a.state.DockerContainers = containers
	if len(containers) == 0 {
		a.state.DockerError = ""
		a.state.SelectedContainerID = ""
		a.state.SelectedContainer = ""
		a.state.ContainerLogs = []string{"No containers"}
		return
	}

	if a.state.SelectedContainerID == "" {
		a.state.SelectedContainerID = containers[0].ID
		a.state.SelectedContainer = containers[0].Name
	}

	var selected models.DockerContainerStat
	found := false
	for _, c := range containers {
		if c.ID == a.state.SelectedContainerID {
			selected = c
			found = true
			break
		}
	}
	if !found {
		selected = containers[0]
		a.state.SelectedContainerID = selected.ID
	}
	a.state.SelectedContainer = selected.Name

	logs, err := a.docker.CollectContainerLogs(ctx, a.state.SelectedContainerID, 30)
	if err != nil {
		a.state.DockerError = err.Error()
		a.state.ContainerLogs = []string{"Log error: " + err.Error()}
		return
	}
	a.state.DockerError = ""
	a.state.ContainerLogs = logs
}

func scheduleMetricsTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return metricsTickMsg(t)
	})
}

func convertCollectorProcesses(items []collectors.ProcessStat) []models.ProcessStat {
	out := make([]models.ProcessStat, 0, len(items))
	for _, p := range items {
		out = append(out, models.ProcessStat{
			PID:        p.PID,
			Name:       p.Name,
			CPUPercent: p.CPUPercent,
			MemoryMB:   p.MemoryMB,
		})
	}
	return out
}

// View renders the UI (bubbletea Model interface)
func (a *App) View() string {
	if a.state.Width == 0 || a.state.Height == 0 {
		return "Initializing..."
	}

	// Render appropriate view based on mode
	switch a.state.Mode {
	case models.ModeNormal:
		return ui.RenderDashboard(a.state, a.theme)
	case models.ModeModal:
		dashboard := ui.RenderDashboard(a.state, a.theme)
		modal := ui.RenderModal(a.state, a.theme)
		return lipgloss.JoinVertical(lipgloss.Left, dashboard, modal)
	case models.ModePaused:
		pauseOverlay := a.renderPauseOverlay()
		dashboard := ui.RenderDashboard(a.state, a.theme)
		// Overlay pause text over dashboard
		return a.overlayText(dashboard, pauseOverlay)
	case models.ModeFocused:
		// TODO: Phase 4 - Implement focused panel view
		return ui.RenderDashboard(a.state, a.theme)
	default:
		return ui.RenderDashboard(a.state, a.theme)
	}
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(strings.TrimSpace(msg.String()))

	// Global Back key: always return to dashboard from sub-modes.
	// Ignore global back when inline editing in the ping modal to allow typing 'b'.
	if !a.state.PingEditing && isBackKey(msg, key) {
		switch a.state.Mode {
		case models.ModeModal:
			a.state.CloseModal()
		case models.ModeFocused, models.ModePaused:
			a.state.SetMode(models.ModeNormal)
			a.state.IsPaused = false
		}
		return a, tea.ClearScreen
	}

	// Modal-first behavior: allow users to reliably go back.
	if a.state.Mode == models.ModeModal {
		if a.state.ActiveModal == models.ModalDocker {
			if handled, cmd := a.handleDockerModalKey(msg, key); handled {
				return a, cmd
			}
		}
		if a.state.ActiveModal == models.ModalPing {
			if handled, cmd := a.handlePingModalKey(msg, key); handled {
				return a, cmd
			}
		}
		if isEscapeKey(msg, key) {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
		if msg.Type == tea.KeyEnter && a.state.ActiveModal != models.ModalDocker {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
		if key == "q" && !a.state.PingEditing {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
	}

	switch key {
	// Quit
	case "q":
		if a.state.PingEditing {
			return a, nil
		}
		return a, tea.Quit

	// Pause/Resume
	case "p":
		a.state.TogglePause()
		return a, nil
	case "s":
		a.state.StorageListView = !a.state.StorageListView
		return a, nil

	// F-keys for modals
	case "f1":
		a.toggleModal(models.ModalRefreshRate)
		return a, tea.ClearScreen
	case "f2":
		a.toggleModal(models.ModalDocker)
		a.syncDockerModalSelection()
		return a, tea.ClearScreen
	case "f3":
		a.toggleModal(models.ModalPing)
		return a, tea.ClearScreen
	case "f4":
		// Open focus grid selector modal (Phase 1 stub)
		if a.state.Mode == models.ModeFocused {
			a.state.SetMode(models.ModeNormal)
		} else {
			a.toggleModal(models.ModalFocus)
		}
		return a, tea.ClearScreen
	case "f5":
		a.toggleModal(models.ModalTheme)
		return a, tea.ClearScreen
	case "f6":
		a.toggleModal(models.ModalAlerts)
		return a, tea.ClearScreen
	case "f7":
		a.toggleModal(models.ModalExport)
		return a, tea.ClearScreen

	// Close modal with ESC
	case "esc":
		if a.state.Mode == models.ModeModal {
			a.state.CloseModal()
		}
		return a, tea.ClearScreen

	// Other keys
	case "r":
		// Force refresh (Phase 2+)
		return a, nil
	case "tab":
		// Cycle focus between panels
		return a, nil
	case "?":
		// Help (Phase 7+)
		return a, nil
	}

	return a, nil
}

func (a *App) toggleModal(modalType models.ModalType) {
	if a.state.Mode == models.ModeModal && a.state.ActiveModal == modalType {
		a.state.CloseModal()
		return
	}
	a.state.OpenModal(modalType)
}

func (a *App) syncDockerModalSelection() {
	if len(a.state.DockerContainers) == 0 {
		a.state.DockerModalIndex = 0
		return
	}

	a.state.DockerModalIndex = 0
	for i, c := range a.state.DockerContainers {
		if c.ID == a.state.SelectedContainerID {
			a.state.DockerModalIndex = i
			return
		}
	}
}

func (a *App) handleDockerModalKey(msg tea.KeyMsg, key string) (bool, tea.Cmd) {
	if len(a.state.DockerContainers) == 0 {
		return false, nil
	}

	switch key {
	case "up", "k":
		if a.state.DockerModalIndex > 0 {
			a.state.DockerModalIndex--
		}
		return true, tea.ClearScreen
	case "down", "j":
		if a.state.DockerModalIndex < len(a.state.DockerContainers)-1 {
			a.state.DockerModalIndex++
		}
		return true, tea.ClearScreen
	}

	if msg.Type == tea.KeyEnter {
		idx := a.state.DockerModalIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(a.state.DockerContainers) {
			idx = len(a.state.DockerContainers) - 1
		}
		selected := a.state.DockerContainers[idx]
		a.state.SelectedContainerID = selected.ID
		a.state.SelectedContainer = selected.Name
		a.state.CloseModal()
		a.collectDockerSnapshot()
		return true, tea.ClearScreen
	}

	return false, nil
}

func (a *App) handlePingModalKey(msg tea.KeyMsg, key string) (bool, tea.Cmd) {
	// Load path helper
	getPath := func() string {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".config", "tshoot", "ping_targets.toml")
		}
		return "./ping_targets.toml"
	}

	// If inline-editing active, handle character input and editing keys
	if a.state.PingEditing {
		// typing characters
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			ch := msg.Runes[0]
			if a.state.PingEditField == 0 {
				a.state.PingEditLabel += string(ch)
			} else {
				a.state.PingEditHost += string(ch)
			}
			return true, nil
		}
		// handle control keys
		if msg.Type == tea.KeyBackspace || key == "backspace" || key == "delete" {
			if a.state.PingEditField == 0 {
				a.state.PingEditLabel = trimLastRune(a.state.PingEditLabel)
			} else {
				a.state.PingEditHost = trimLastRune(a.state.PingEditHost)
			}
			return true, nil
		}
		if key == "tab" {
			a.state.PingEditField = 1 - a.state.PingEditField
			return true, nil
		}
		if msg.Type == tea.KeyEnter || key == "enter" {
			// persist target
			path := getPath()
			var targets []collectors.PingTarget
			if t, err := collectors.LoadPingTargetsFromFile(path); err == nil {
				targets = t
			} else {
				targets = a.pingTargets
			}
			if a.state.PingEditIndex >= 0 && a.state.PingEditIndex < len(targets) {
				targets[a.state.PingEditIndex] = collectors.PingTarget{Label: a.state.PingEditLabel, Host: a.state.PingEditHost}
			} else {
				targets = append(targets, collectors.PingTarget{Label: a.state.PingEditLabel, Host: a.state.PingEditHost})
			}
			targets = collectors.EnsureDefaultPingTargets(targets)
			_ = collectors.SavePingTargetsToFile(path, targets)
			// reload into memory
			pts := make([]collectors.PingTarget, 0, len(targets))
			for _, t := range targets {
				pts = append(pts, collectors.PingTarget{Label: t.Label, Host: t.Host})
			}
			a.pingTargets = pts
			a.state.PingEditing = false
			a.state.PingEditIndex = -1
			a.state.PingEditLabel = ""
			a.state.PingEditHost = ""
			a.state.PingEditField = 0
			return true, tea.ClearScreen
		}
		if msg.Type == tea.KeyEsc || key == "esc" {
			// cancel
			a.state.PingEditing = false
			a.state.PingEditIndex = -1
			a.state.PingEditLabel = ""
			a.state.PingEditHost = ""
			a.state.PingEditField = 0
			return true, tea.ClearScreen
		}
		return true, nil
	}

	// navigation
	switch key {
	case "up", "k":
		if a.state.PingModalIndex > 0 {
			a.state.PingModalIndex--
		}
		return true, tea.ClearScreen
	case "down", "j":
		if a.state.PingModalIndex < len(a.pingTargets)-1 {
			a.state.PingModalIndex++
		}
		return true, tea.ClearScreen
	case "+", "=", "a":
		// start inline add flow without removing defaults
		a.state.PingEditing = true
		a.state.PingEditIndex = -1
		a.state.PingEditLabel = ""
		a.state.PingEditHost = ""
		a.state.PingEditField = 0
		return true, tea.ClearScreen
	case "-", "d":
		// remove selected from existing targets
		path := getPath()
		targets, err := collectors.LoadPingTargetsFromFile(path)
		if err != nil {
			targets = a.pingTargets
		}
		if len(targets) == 0 {
			return true, tea.ClearScreen
		}
		idx := a.state.PingModalIndex
		if idx < 0 || idx >= len(targets) {
			idx = 0
		}
		if collectors.IsDefaultPingTarget(targets[idx]) {
			return true, tea.ClearScreen
		}
		targets = append(targets[:idx], targets[idx+1:]...)
		targets = collectors.EnsureDefaultPingTargets(targets)
		_ = collectors.SavePingTargetsToFile(path, targets)
		// reload
		pts := make([]collectors.PingTarget, 0, len(targets))
		for _, t := range targets {
			pts = append(pts, collectors.PingTarget{Label: t.Label, Host: t.Host})
		}
		a.pingTargets = pts
		if a.state.PingModalIndex >= len(a.pingTargets) && a.state.PingModalIndex > 0 {
			a.state.PingModalIndex--
		}
		return true, tea.ClearScreen
	case "enter":
		if len(a.pingTargets) > 0 {
			idx := a.state.PingModalIndex
			if idx < 0 {
				idx = 0
			}
			if idx >= len(a.pingTargets) {
				idx = len(a.pingTargets) - 1
			}
			a.state.PingEditing = true
			a.state.PingEditIndex = idx
			a.state.PingEditLabel = a.pingTargets[idx].Label
			a.state.PingEditHost = a.pingTargets[idx].Host
			a.state.PingEditField = 0
		}
		return true, tea.ClearScreen
	case "e":
		// open file in $EDITOR
		path := getPath()
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		cmd := exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		// reload and preserve defaults
		if targets, err := collectors.LoadPingTargetsFromFile(path); err == nil {
			targets = collectors.EnsureDefaultPingTargets(targets)
			pts := make([]collectors.PingTarget, 0, len(targets))
			for _, t := range targets {
				pts = append(pts, collectors.PingTarget{Label: t.Label, Host: t.Host})
			}
			a.pingTargets = pts
		}
		return true, tea.ClearScreen
	}

	return false, nil
}

func isEscapeKey(msg tea.KeyMsg, key string) bool {
	if msg.Type == tea.KeyEsc {
		return true
	}

	switch key {
	case "esc", "escape", "ctrl+[":
		return true
	default:
		return false
	}
}

func isBackKey(msg tea.KeyMsg, key string) bool {
	if key == "b" || key == "back" {
		return true
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r == 'b' || r == 'B' {
			return true
		}
	}

	return false
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

// renderPauseOverlay renders a pause indicator overlay
func (a *App) renderPauseOverlay() string {
	pauseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Background(lipgloss.Color("17")).
		Bold(true).
		Padding(1, 3)

	return pauseStyle.Render("⏸ PAUSED - Press 'p' to resume")
}

// overlayText overlays one text on top of another (center)
func (a *App) overlayText(background, overlay string) string {
	// For now, just append the overlay - proper overlay logic in Phase 7
	return strings.TrimRight(background, "\n") + "\n" + overlay
}

// main is the entry point
func main() {
	// Create app
	app := NewApp()

	// Create bubbletea program
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		log.Fatal(err)
	}
}
