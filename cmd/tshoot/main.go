package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/tshoot/internal/collectors"
	"github.com/user/tshoot/internal/models"
	"github.com/user/tshoot/internal/ui"
)

// App is the main application model
type App struct {
	state   *models.AppState
	config  *Config
	theme   *ui.Theme
	cpuCalc *collectors.CPUCalculator
	procCalc *collectors.ProcessCollector
	pingTargets []collectors.PingTarget
	docker *collectors.DockerCollector
}

type metricsTickMsg time.Time

// NewApp creates a new application
func NewApp() *App {
	config := LoadConfig()
	state := models.NewAppState(80, 24) // Default size, will be updated on first render

	theme := ui.GetTheme(config.General.Theme)
	dockerCollector, _ := collectors.NewDockerCollector()

	return &App{
		state:   state,
		config:  config,
		theme:   theme,
		cpuCalc: &collectors.CPUCalculator{},
		procCalc: collectors.NewProcessCollector(),
		pingTargets: []collectors.PingTarget{
			{Label: "Google DNS", Host: "8.8.8.8"},
			{Label: "Cloudflare", Host: "1.1.1.1"},
		},
		docker: dockerCollector,
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
	if isBackKey(msg, key) {
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
		if isEscapeKey(msg, key) {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
		if msg.Type == tea.KeyEnter {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
		if key == "q" {
			a.state.CloseModal()
			return a, tea.ClearScreen
		}
	}

	switch key {
	// Quit
	case "q":
		return a, tea.Quit

	// Pause/Resume
	case "p":
		a.state.TogglePause()
		return a, nil

	// F-keys for modals
	case "f1":
		a.toggleModal(models.ModalRefreshRate)
		return a, tea.ClearScreen
	case "f2":
		a.toggleModal(models.ModalDocker)
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
	case "/":
		// Search (Phase 5+)
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
