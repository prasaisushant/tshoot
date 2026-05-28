package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/prasaisushant/tshoot/internal/models"
)

// Theme holds color and style definitions
type Theme struct {
	Name          string
	PanelBorder   lipgloss.Border
	PanelBorderFg lipgloss.Color
	PanelBg       lipgloss.Color
	TextFg        lipgloss.Color
	AccentFg      lipgloss.Color
	WarningFg     lipgloss.Color
	CriticalFg    lipgloss.Color
}

// GetTheme returns the theme based on the name
func GetTheme(name string) *Theme {
	switch name {
	case "light":
		return &Theme{
			Name:          "light",
			PanelBorder:   lipgloss.RoundedBorder(),
			PanelBorderFg: lipgloss.Color("238"),
			PanelBg:       lipgloss.Color("255"),
			TextFg:        lipgloss.Color("16"),
			AccentFg:      lipgloss.Color("33"),
			WarningFg:     lipgloss.Color("214"),
			CriticalFg:    lipgloss.Color("196"),
		}
	default: // "dark"
		return &Theme{
			Name:          "dark",
			PanelBorder:   lipgloss.RoundedBorder(),
			PanelBorderFg: lipgloss.Color("240"),
			PanelBg:       lipgloss.Color("236"),
			TextFg:        lipgloss.Color("250"),
			AccentFg:      lipgloss.Color("86"),
			WarningFg:     lipgloss.Color("208"),
			CriticalFg:    lipgloss.Color("196"),
		}
	}
}

// PanelStyle returns a lipgloss style for a panel box
func (t *Theme) PanelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.PanelBorder).
		BorderForeground(t.PanelBorderFg).
		Foreground(t.TextFg).
		Background(t.PanelBg).
		Padding(0, 1).
		Width(width)
}

// TitleStyle returns a lipgloss style for panel titles
func (t *Theme) TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.AccentFg).
		Bold(true)
}

// Panel represents a dashboard panel with title and content
type Panel struct {
	Title     string
	Content   []string
	Width     int
	Height    int
	IsFocused bool
}

// RenderPanel renders a panel with the given theme
func RenderPanel(panel *Panel, theme *Theme) string {
	style := theme.PanelStyle(panel.Width)
	frameWidth := style.GetHorizontalFrameSize()
	frameHeight := style.GetVerticalFrameSize()
	contentWidth := max(1, panel.Width-frameWidth)
	contentHeight := max(1, panel.Height-frameHeight)
	titleStyle := theme.TitleStyle()

	lines := make([]string, 0, contentHeight)
	title := truncateToWidth(fmt.Sprintf(" %s ", panel.Title), contentWidth)
	lines = append(lines, titleStyle.Render(title))

	for _, line := range panel.Content {
		if len(lines) >= contentHeight {
			break
		}
		lines = append(lines, truncateToWidth(line, contentWidth))
	}

	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	contentStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight)

	return style.Render(contentStyle.Render(strings.Join(lines, "\n")))
}

// RenderFKeyBar renders the function key bar at the bottom
func RenderFKeyBar(theme *Theme, width int) string {
	if width < 20 {
		return ""
	}

	keys := []string{
		"F1:Refresh",
		"F2:Docker",
		"F3:Ping",
		"F4:Focus",
		"F5:Theme",
		"F6:Alerts",
		"F7:Export",
		"r:Reload",
		"p:Pause",
		"/:Search",
		"Tab:Next",
		"?:Help",
		"b:Back",
		"q:Quit",
	}

	// Build key string with adaptive truncation
	keyStr := strings.Join(keys, "  ")
	maxWidth := width - 4 // account for brackets/spaces

	if maxWidth > 3 && len(keyStr) > maxWidth {
		keyStr = truncateToWidth(keyStr, maxWidth-3) + "..."
	}

	barStyle := lipgloss.NewStyle().
		Foreground(theme.TextFg).
		Background(theme.PanelBg).
		Width(width)

	return barStyle.Render("[ " + keyStr + " ]")
}

// RenderDashboard renders the entire dashboard layout
func RenderDashboard(state *models.AppState, theme *Theme) string {
	if state.Width < 40 || state.Height < 12 {
		return "Terminal too small. Resize to at least 40x12."
	}

	usableWidth := state.Width
	usableHeight := state.Height - 1 // reserve last line for function key bar

	rowHeights := splitDimension(usableHeight, 3)
	row1Height, row2Height, row3Height := rowHeights[0], rowHeights[1], rowHeights[2]
	col4Widths := splitDimension(usableWidth-3, 4) // 3 spaces between 4 panels
	col3Widths := splitDimension(usableWidth-2, 3) // 2 spaces between 3 panels
	cpuBar := renderPercentBar(state.Metrics.CPUPercent, 10)
	memBar := renderPercentBar(state.Metrics.MemoryPercent, 10)
	swapLine := fmt.Sprintf("Swap:  %.0f%% (%d/%dMB)", state.Metrics.SwapPercent, state.Metrics.SwapUsedMB, state.Metrics.SwapTotalMB)
	if state.Metrics.SwapTotalMB == 0 {
		swapLine = "Swap:  N/A"
	}

	cpuMemPanel := &Panel{
		Title:  "CPU / Memory",
		Width:  col4Widths[0],
		Height: row1Height,
		Content: []string{
			fmt.Sprintf("CPU:   %s %3.0f%%", cpuBar, state.Metrics.CPUPercent),
			fmt.Sprintf("Mem:   %s %3.0f%%", memBar, state.Metrics.MemoryPercent),
			fmt.Sprintf("Load:  %.2f %.2f %.2f", state.Metrics.Load1, state.Metrics.Load5, state.Metrics.Load15),
			swapLine,
		},
	}

	storagePanel := &Panel{
		Title:  "Storage",
		Width:  col4Widths[1],
		Height: row1Height,
		Content: []string{
			"/      ext4 23%",
			"/home  ext4 67%",
			"sda    1.0T",
		},
	}

	pingPanel := &Panel{
		Title:   "Ping Status",
		Width:   col4Widths[2],
		Height:  row1Height,
		Content: formatPingContent(state.PingResults, state.PingError),
	}

	uptimePanel := &Panel{
		Title:  "System",
		Width:  col4Widths[3],
		Height: row1Height,
		Content: []string{
			"Uptime: " + formatUptime(state.Metrics.UptimeSeconds),
			"Clock:  " + state.Metrics.Clock,
		},
	}
	if state.Metrics.LastErrorSummary != "" {
		uptimePanel.Content = append(uptimePanel.Content, "Data: limited ("+state.Metrics.LastErrorSummary+")")
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(cpuMemPanel, theme),
		" ",
		RenderPanel(storagePanel, theme),
		" ",
		RenderPanel(pingPanel, theme),
		" ",
		RenderPanel(uptimePanel, theme),
	)

	// Row 2: Top CPU | Ports | IP/Routes (3 columns)
	topCPUPanel := &Panel{
		Title:   "Top CPU",
		Width:   col3Widths[0],
		Height:  row2Height,
		Content: formatTopCPUContent(state.TopCPUProcesses, state.ProcessError),
	}

	portsPanel := &Panel{
		Title:   "Open Ports",
		Width:   col3Widths[1],
		Height:  row2Height,
		Content: formatPortsContent(state.OpenPorts, state.NetworkError),
	}

	ipRoutesPanel := &Panel{
		Title:   "IP / Routes",
		Width:   col3Widths[2],
		Height:  row2Height,
		Content: formatIPRouteContent(state.IPRouteLines, state.NetworkError),
	}

	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(topCPUPanel, theme),
		" ",
		RenderPanel(portsPanel, theme),
		" ",
		RenderPanel(ipRoutesPanel, theme),
	)

	// Row 3: Top Mem | Docker | Container Logs (3 columns)
	topMemPanel := &Panel{
		Title:   "Top Mem",
		Width:   col3Widths[0],
		Height:  row3Height,
		Content: formatTopMemContent(state.TopMemProcesses, state.ProcessError),
	}

	dockerPanel := &Panel{
		Title:   "Docker Containers",
		Width:   col3Widths[1],
		Height:  row3Height,
		Content: formatDockerContainersContent(state.DockerContainers, state.DockerError),
	}

	logsPanel := &Panel{
		Title:   "Container Logs",
		Width:   col3Widths[2],
		Height:  row3Height,
		Content: formatContainerLogsContent(state.SelectedContainer, state.ContainerLogs, state.DockerError),
	}

	row3 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(topMemPanel, theme),
		" ",
		RenderPanel(dockerPanel, theme),
		" ",
		RenderPanel(logsPanel, theme),
	)

	// Combine all rows vertically
	dashboard := lipgloss.JoinVertical(lipgloss.Left,
		row1,
		row2,
		row3,
	)

	// Add F-key bar at bottom
	fKeyBar := RenderFKeyBar(theme, state.Width)
	dashboard = lipgloss.JoinVertical(lipgloss.Left,
		dashboard,
		fKeyBar,
	)

	return dashboard
}

func splitDimension(total, parts int) []int {
	if parts <= 0 {
		return nil
	}
	base := total / parts
	rem := total % parts
	out := make([]int, parts)
	for i := 0; i < parts; i++ {
		out[i] = base
		if i < rem {
			out[i]++
		}
		if out[i] < 1 {
			out[i] = 1
		}
	}
	return out
}

func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	for _, r := range runes {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > w {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func renderPercentBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func formatUptime(seconds float64) string {
	if seconds <= 0 {
		return "N/A"
	}

	d := time.Duration(seconds * float64(time.Second))
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}

func formatTopCPUContent(processes []models.ProcessStat, errSummary string) []string {
	if len(processes) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No process data"}
	}

	lines := make([]string, 0, len(processes))
	for _, p := range processes {
		lines = append(lines, fmt.Sprintf("%-14s %5d %4.1f%%", p.Name, p.PID, p.CPUPercent))
	}
	return lines
}

func formatTopMemContent(processes []models.ProcessStat, errSummary string) []string {
	if len(processes) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No process data"}
	}

	lines := make([]string, 0, len(processes))
	for _, p := range processes {
		lines = append(lines, fmt.Sprintf("%-14s %5d %4dMB", p.Name, p.PID, p.MemoryMB))
	}
	return lines
}

func formatPortsContent(ports []models.PortStat, errSummary string) []string {
	if len(ports) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No listening ports"}
	}

	lines := make([]string, 0, len(ports))
	for _, p := range ports {
		pidText := "-"
		if p.PID > 0 {
			pidText = fmt.Sprintf("%d", p.PID)
		}
		lines = append(lines, fmt.Sprintf("%5d %-4s %-6s %s", p.Port, p.Proto, pidText, p.Process))
	}
	return lines
}

func formatIPRouteContent(lines []string, errSummary string) []string {
	if len(lines) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No route data"}
	}
	return lines
}

func formatPingContent(results []models.PingStat, errSummary string) []string {
	if len(results) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No ping targets"}
	}

	lines := make([]string, 0, len(results))
	for _, r := range results {
		label := r.Label
		if label == "" {
			label = r.Host
		}
		if r.Up {
			lines = append(lines, fmt.Sprintf("%-12s \u2713 %.1fms", label, r.LatencyMS))
		} else {
			lines = append(lines, fmt.Sprintf("%-12s \u2717 DOWN", label))
		}
	}

	if errSummary != "" {
		lines = append(lines, "Data: limited")
	}
	return lines
}

func formatDockerContainersContent(containers []models.DockerContainerStat, errSummary string) []string {
	if len(containers) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No containers"}
	}

	lines := make([]string, 0, len(containers))
	for _, c := range containers {
		status := c.Status
		if lipgloss.Width(status) > 12 {
			status = truncateToWidth(status, 12)
		}
		lines = append(lines, fmt.Sprintf("%-14s %-12s", c.Name, status))
	}
	return lines
}

func formatContainerLogsContent(selected string, logs []string, errSummary string) []string {
	if errSummary != "" && len(logs) == 0 {
		return []string{"Data: limited (" + errSummary + ")"}
	}

	out := make([]string, 0, len(logs)+1)
	if selected != "" {
		out = append(out, "["+selected+"]")
	}
	if len(logs) == 0 {
		out = append(out, "No logs")
		return out
	}

	start := 0
	if len(logs) > 6 {
		start = len(logs) - 6
	}
	out = append(out, logs[start:]...)
	return out
}

// RenderModal renders a modal overlay (stub for Phase 1)
func RenderModal(state *models.AppState, theme *Theme) string {
	switch state.ActiveModal {
	case models.ModalRefreshRate:
		return renderRefreshRateModal(theme, state.Width, state.Height)
	case models.ModalDocker:
		return renderDockerModal(state, theme, state.Width, state.Height)
	case models.ModalPing:
		return renderPingModal(theme, state.Width, state.Height)
	case models.ModalFocus:
		return renderFocusModal(theme, state.Width, state.Height)
	case models.ModalTheme:
		return renderThemeModal(theme, state.Width, state.Height)
	case models.ModalAlerts:
		return renderAlertsModal(theme, state.Width, state.Height)
	case models.ModalExport:
		return renderExportModal(theme, state.Width, state.Height)
	default:
		return ""
	}
}

// Stub modal renderers for Phase 1
func renderRefreshRateModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Refresh Rate Settings", []string{
		"○ 1s   (live)",
		"● 3s   (default)",
		"○ 5s",
		"○ 10s",
	}, width, height)
}

func renderDockerModal(state *models.AppState, theme *Theme, width, height int) string {
	lines := make([]string, 0, len(state.DockerContainers)+3)
	lines = append(lines, "Use ↑/↓ (or j/k), Enter to select")
	lines = append(lines, "")

	if len(state.DockerContainers) == 0 {
		if state.DockerError != "" {
			lines = append(lines, "Docker unavailable: "+state.DockerError)
		} else {
			lines = append(lines, "No containers")
		}
		return centerModal(theme, "Docker Containers", lines, width, height)
	}

	for i, c := range state.DockerContainers {
		prefix := "○"
		if i == state.DockerModalIndex {
			prefix = "●"
		}
		lines = append(lines, fmt.Sprintf("%s %-14s %s", prefix, c.Name, c.Status))
	}
	return centerModal(theme, "Docker Containers", lines, width, height)
}

func renderPingModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Ping Targets", []string{
		"8.8.8.8        ✓ 12ms",
		"1.1.1.1        ✓  8ms",
		"Internal API   ✗ DOWN",
	}, width, height)
}

func renderThemeModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Theme", []string{
		"● Dark",
		"○ Light",
		"○ High Contrast",
	}, width, height)
}

func renderFocusModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Focus Panel (F4)", []string{
		"Use arrows to select a panel",
		"Press Enter to focus full-screen",
		"ESC to return",
		"",
		"[ CPU ] [ Storage ] [ Ping ] [ System ]",
		"[ Top CPU ] [ Ports ] [ IP/Routes ]",
		"[ Top Mem ] [ Docker ] [ Logs ]",
	}, width, height)
}

func renderAlertsModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Alert Thresholds", []string{
		"CPU Warn:   80%",
		"CPU Crit:   95%",
		"Mem Warn:   80%",
		"Disk Warn:  90%",
		"",
		"[Phase 1 placeholder]",
	}, width, height)
}

func renderExportModal(theme *Theme, width, height int) string {
	return centerModal(theme, "Export Snapshot", []string{
		"Output format:",
		"● .txt",
		"○ .json",
		"",
		"Press Enter to export",
		"[Phase 1 placeholder]",
	}, width, height)
}

// centerModal is a helper to render a centered modal
func centerModal(theme *Theme, title string, lines []string, width, height int) string {
	modalWidth := width / 2
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	modalHeight := len(lines) + 4
	if modalHeight > height-4 {
		modalHeight = height - 4
	}

	titleStyle := theme.TitleStyle()
	style := theme.PanelStyle(modalWidth)

	content := titleStyle.Render(title) + "\n"
	for _, line := range lines {
		if len(line) > modalWidth-4 {
			line = line[:modalWidth-4]
		}
		content += line + "\n"
	}

	modalBox := style.Render(content)

	// Center the modal
	totalWidth := width
	totalHeight := height
	boxLines := strings.Split(modalBox, "\n")
	centeredLines := make([]string, 0)

	topPadding := (totalHeight - len(boxLines)) / 2
	for i := 0; i < topPadding; i++ {
		centeredLines = append(centeredLines, "")
	}

	for _, line := range boxLines {
		leftPadding := (totalWidth - len(line)) / 2
		if leftPadding < 0 {
			leftPadding = 0
		}
		centeredLines = append(centeredLines, strings.Repeat(" ", leftPadding)+line)
	}

	return strings.Join(centeredLines, "\n")
}
