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
	if panel.IsFocused {
		style = style.BorderForeground(theme.AccentFg).Foreground(theme.AccentFg)
	}
	frameWidth := style.GetHorizontalFrameSize()
	frameHeight := style.GetVerticalFrameSize()
	contentWidth := max(1, panel.Width-frameWidth)
	contentHeight := max(1, panel.Height-frameHeight)
	titleStyle := theme.TitleStyle()

	if panel.IsFocused {
		titleStyle = titleStyle.Bold(true)
	}

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
		"r:Reset",
		"s:Storage",
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

	storageTitle := "Storage"
	if state.StorageListView {
		storageTitle += " (list)"
	}
	storageStyle := theme.PanelStyle(col4Widths[1])
	storageContentWidth := max(1, col4Widths[1]-storageStyle.GetHorizontalFrameSize())
	storagePanel := &Panel{
		Title:   storageTitle,
		Width:   col4Widths[1],
		Height:  row1Height,
		Content: formatStorageContent(state.StorageMounts, state.StorageDeviceEntries, state.StorageError, state.StorageIOReadKB, state.StorageIOWriteKB, state.StorageLoopCount, state.StorageListView, storageContentWidth),
	}

	pingPanel := &Panel{
		Title:   "Ping And DNS Status",
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

func renderPieChart(percent float64, width int) string {
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
	return strings.Repeat("●", filled) + strings.Repeat("○", width-filled)
}

func formatFocusCPUMemoryContent(state *models.AppState, width int) []string {
	if width < 10 {
		width = 10
	}
	lines := []string{
		fmt.Sprintf("CPU Usage:  %3.0f%%", state.Metrics.CPUPercent),
		renderPercentBar(state.Metrics.CPUPercent, width-2),
		"",
		fmt.Sprintf("Mem Usage:  %3.0f%%", state.Metrics.MemoryPercent),
		renderPercentBar(state.Metrics.MemoryPercent, width-2),
		"",
		fmt.Sprintf("Mem Pie: [%s]", renderPieChart(state.Metrics.MemoryPercent, 10)),
		"",
		fmt.Sprintf("Load: %.2f %.2f %.2f", state.Metrics.Load1, state.Metrics.Load5, state.Metrics.Load15),
		fmt.Sprintf("Clock: %s", state.Metrics.Clock),
		fmt.Sprintf("Refresh: every %ds", state.RefreshIntervalSec),
		"",
		"Back: b   Reset: r",
	}
	return lines
}

func formatFocusStorageContent(state *models.AppState, width int) []string {
	return formatStorageContent(state.StorageMounts, state.StorageDeviceEntries, state.StorageError, state.StorageIOReadKB, state.StorageIOWriteKB, state.StorageLoopCount, true, width)
}

func formatIPRouteContent(lines []string, errSummary string) []string {
	if len(lines) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No network data"}
	}

	ipLines := []string{"IP Addresses:", strings.Repeat("─", 18)}
	routeLines := []string{"Routes:", strings.Repeat("─", 18)}
	for _, line := range lines {
		if strings.HasPrefix(line, "gw ") {
			routeLines = append(routeLines, line)
		} else {
			ipLines = append(ipLines, line)
		}
	}
	if len(ipLines) == 2 {
		ipLines = append(ipLines, "No IP addresses")
	}
	if len(routeLines) == 2 {
		routeLines = append(routeLines, "No default route")
	}

	out := make([]string, 0, len(ipLines)+len(routeLines)+1)
	out = append(out, ipLines...)
	out = append(out, "")
	out = append(out, routeLines...)
	if errSummary != "" {
		out = append(out, "", "Data: limited ("+errSummary+")")
	}
	return out
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

	lines := make([]string, 0, len(processes)+1)
	lines = append(lines, fmt.Sprintf("%-14s %5s %6s", "PROCESS", "PID", "CPU"))
	lines = append(lines, strings.Repeat("─", 28))
	for _, p := range processes {
		lines = append(lines, fmt.Sprintf("%-14s %5d %5.1f%%", p.Name, p.PID, p.CPUPercent))
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

	lines := make([]string, 0, len(processes)+1)
	lines = append(lines, fmt.Sprintf("%-14s %5s %8s", "PROCESS", "PID", "MEM"))
	lines = append(lines, strings.Repeat("─", 28))
	for _, p := range processes {
		lines = append(lines, fmt.Sprintf("%-14s %5d %7dMB", p.Name, p.PID, p.MemoryMB))
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

	lines := make([]string, 0, len(ports)+2)
	lines = append(lines, fmt.Sprintf("%5s %-4s %-6s %s", "PORT", "PROTO", "PID", "PROCESS"))
	lines = append(lines, strings.Repeat("─", 30))
	for _, p := range ports {
		pidText := "-"
		if p.PID > 0 {
			pidText = fmt.Sprintf("%d", p.PID)
		}
		lines = append(lines, fmt.Sprintf("%5d %-4s %-6s %s", p.Port, p.Proto, pidText, p.Process))
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

	lines := make([]string, 0, len(results)+2)
	lines = append(lines, fmt.Sprintf("%-14s %-8s %s", "LABEL", "STATUS", "LATENCY"))
	lines = append(lines, strings.Repeat("─", 36))
	for _, r := range results {
		label := r.Label
		if label == "" {
			label = r.Host
		}
		if r.Up {
			lines = append(lines, fmt.Sprintf("%-14s %-8s %5.1fms", label, "UP", r.LatencyMS))
		} else {
			lines = append(lines, fmt.Sprintf("%-14s %-8s %s", label, "DOWN", "-"))
		}
	}

	if errSummary != "" {
		lines = append(lines, "Data: limited")
	}
	return lines
}

func formatStorageContent(mounts []models.StorageMount, devices []models.StorageDeviceEntry, errSummary string, readKB, writeKB float64, loopCount int, listView bool, width int) []string {
	ioLine := formatStorageIOLine(readKB, writeKB)
	if listView {
		if len(devices) == 0 {
			if errSummary != "" {
				return []string{"Data: limited (" + errSummary + ")"}
			}
			return []string{"No device data"}
		}

		lines := make([]string, 0, len(devices)+4)
		lines = append(lines, fmt.Sprintf("%-8s %-6s %s", "DEVICE", "TYPE", "SIZE"))
		lines = append(lines, strings.Repeat("─", width-4))
		for _, d := range devices {
			lines = append(lines, formatDeviceLine(d, width))
		}
		if loopCount > 0 {
			lines = append(lines, fmt.Sprintf("loop*: %d devices", loopCount))
		}
		if ioLine != "" {
			lines = append(lines, ioLine)
		}
		if errSummary != "" {
			lines = append(lines, "Data: limited ("+errSummary+")")
		}
		return lines
	}

	if len(mounts) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No mount data"}
	}

	lines := make([]string, 0, len(mounts)+4)
	lines = append(lines, fmt.Sprintf("%-20s %-6s %5s %8s", "MOUNT", "FSTYPE", "USE%", "SIZE"))
	lines = append(lines, strings.Repeat("─", width-4))
	for _, m := range mounts {
		lines = append(lines, formatStorageLine(m, width))
	}
	if loopCount > 0 {
		lines = append(lines, fmt.Sprintf("loop*: %d devices", loopCount))
	}
	if ioLine != "" {
		lines = append(lines, ioLine)
	}
	if errSummary != "" {
		lines = append(lines, "Data: limited ("+errSummary+")")
	}
	return lines
}

func formatStorageLine(entry models.StorageMount, width int) string {
	rightFields := fmt.Sprintf("%-6s %5s %8s", entry.FSType, entry.UsePct, entry.Size)
	if width <= lipgloss.Width(rightFields)+1 {
		return truncateToWidth(rightFields, width)
	}

	mountWidth := width - lipgloss.Width(rightFields) - 1
	return fmt.Sprintf("%-*s %s", mountWidth, truncateMiddle(entry.Mount, mountWidth), rightFields)
}

func formatDeviceLine(entry models.StorageDeviceEntry, width int) string {
	if width < 24 {
		return truncateToWidth(fmt.Sprintf("%s %s %s", entry.Name, entry.Type, entry.Size), width)
	}

	rightFields := fmt.Sprintf("%-6s %6s", entry.Type, entry.Size)
	mountWidth := width - lipgloss.Width(rightFields) - lipgloss.Width(entry.Name) - 2
	if mountWidth < 0 {
		mountWidth = 0
	}
	return fmt.Sprintf("%-8s %s %s", entry.Name, rightFields, truncateMiddle(entry.Mount, mountWidth))
}

func formatStorageIOLine(readKB, writeKB float64) string {
	if readKB <= 0 && writeKB <= 0 {
		return "I/O: warming up"
	}
	readText := formatSizeWithUnit(readKB)
	writeText := formatSizeWithUnit(writeKB)
	return fmt.Sprintf("I/O: R %s/s  W %s/s", readText, writeText)
}

func formatSizeWithUnit(kb float64) string {
	if kb >= 1024 {
		return fmt.Sprintf("%.1fMB", kb/1024)
	}
	return fmt.Sprintf("%.0fKB", kb)
}

func truncateMiddle(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 6 {
		return truncateToWidth(s, width)
	}

	leftWidth := (width - 3) / 2
	rightWidth := width - 3 - leftWidth
	left := truncateToWidth(s, leftWidth)

	runes := []rune(s)
	var suffix []rune
	currentWidth := 0
	for i := len(runes) - 1; i >= 0; i-- {
		runeWidth := lipgloss.Width(string(runes[i]))
		if currentWidth+runeWidth > rightWidth {
			break
		}
		suffix = append([]rune{runes[i]}, suffix...)
		currentWidth += runeWidth
	}

	return left + "..." + string(suffix)
}

func formatDockerContainersContent(containers []models.DockerContainerStat, errSummary string) []string {
	if len(containers) == 0 {
		if errSummary != "" {
			return []string{"Data: limited (" + errSummary + ")"}
		}
		return []string{"No containers"}
	}

	lines := make([]string, 0, len(containers)+2)
	lines = append(lines, fmt.Sprintf("%-14s %s", "CONTAINER", "STATUS"))
	lines = append(lines, strings.Repeat("─", 28))
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

// RenderModal renders a modal overlay
func RenderModal(state *models.AppState, theme *Theme) string {
	switch state.ActiveModal {
	case models.ModalRefreshRate:
		return renderRefreshRateModal(state, theme, state.Width, state.Height)
	case models.ModalDocker:
		return renderDockerModal(state, theme, state.Width, state.Height)
	case models.ModalPing:
		return renderPingModal(state, theme, state.Width, state.Height)
	case models.ModalFocus:
		return renderFocusModal(theme, state.Width, state.Height)
	default:
		return ""
	}
}

// RenderFocusedDashboard renders an expanded focus view
func RenderFocusedDashboard(state *models.AppState, theme *Theme) string {
	if state.Width < 60 || state.Height < 18 {
		return "Terminal too small for focus view. Resize to at least 60x18."
	}

	if !state.FocusPanelSelected {
		return renderFocusSelectionDashboard(state, theme)
	}

	return renderFocusedPanel(state, theme)
}

func renderFocusSelectionDashboard(state *models.AppState, theme *Theme) string {
	usableWidth := state.Width
	usableHeight := state.Height - 2
	panelWidth := splitDimension(usableWidth-3, 4)[0]
	summaryHeight := max(6, usableHeight/3)

	panels := []*Panel{
		{
			Title:     "CPU/Mem",
			Width:     panelWidth,
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 0,
			Content: []string{
				fmt.Sprintf("CPU: %3.0f%%", state.Metrics.CPUPercent),
				fmt.Sprintf("Mem: %3.0f%%", state.Metrics.MemoryPercent),
				fmt.Sprintf("Swap: %3.0f%%", state.Metrics.SwapPercent),
			},
		},
		{
			Title:     "Storage",
			Width:     panelWidth,
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 1,
			Content: []string{
				fmt.Sprintf("Mounts: %d", len(state.StorageMounts)),
				fmt.Sprintf("R: %s/s", formatSizeWithUnit(state.StorageIOReadKB)),
				fmt.Sprintf("W: %s/s", formatSizeWithUnit(state.StorageIOWriteKB)),
			},
		},
		{
			Title:     "Ping",
			Width:     panelWidth,
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 2,
			Content: []string{
				fmt.Sprintf("Targets: %d", len(state.PingResults)),
				fmt.Sprintf("Errors: %s", state.PingError),
			},
		},
		{
			Title:     "Uptime",
			Width:     panelWidth,
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 3,
			Content: []string{
				formatUptime(state.Metrics.UptimeSeconds),
				state.Metrics.Clock,
				fmt.Sprintf("Load: %.2f %.2f %.2f", state.Metrics.Load1, state.Metrics.Load5, state.Metrics.Load15),
			},
		},
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(panels[0], theme),
		" ",
		RenderPanel(panels[1], theme),
		" ",
		RenderPanel(panels[2], theme),
		" ",
		RenderPanel(panels[3], theme),
	)

	row2Widths := splitDimension(usableWidth-2, 3)
	panels2 := []*Panel{
		{
			Title:     "Top CPU",
			Width:     row2Widths[0],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 4,
			Content:   previewTopCPUContent(state.TopCPUProcesses),
		},
		{
			Title:     "Ports",
			Width:     row2Widths[1],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 5,
			Content:   previewPortsContent(state.OpenPorts),
		},
		{
			Title:     "IP / Routes",
			Width:     row2Widths[2],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 6,
			Content:   previewIPRouteContent(state.IPRouteLines),
		},
	}

	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(panels2[0], theme),
		" ",
		RenderPanel(panels2[1], theme),
		" ",
		RenderPanel(panels2[2], theme),
	)

	row3Widths := splitDimension(usableWidth-2, 3)
	panels3 := []*Panel{
		{
			Title:     "Top Mem",
			Width:     row3Widths[0],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 7,
			Content:   previewTopMemContent(state.TopMemProcesses),
		},
		{
			Title:     "Docker",
			Width:     row3Widths[1],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 8,
			Content: []string{
				fmt.Sprintf("Containers: %d", len(state.DockerContainers)),
				state.SelectedContainer,
			},
		},
		{
			Title:     "Logs",
			Width:     row3Widths[2],
			Height:    summaryHeight,
			IsFocused: state.FocusedPanel == 9,
			Content:   previewLogsContent(state.ContainerLogs),
		},
	}

	row3 := lipgloss.JoinHorizontal(lipgloss.Top,
		RenderPanel(panels3[0], theme),
		" ",
		RenderPanel(panels3[1], theme),
		" ",
		RenderPanel(panels3[2], theme),
	)

	dashboard := lipgloss.JoinVertical(lipgloss.Left,
		theme.TitleStyle().Render("↑↓←→ to select panel, Enter to focus"),
		"",
		row1,
		"",
		row2,
		"",
		row3,
		"",
		theme.TitleStyle().Render("Press Enter to open selected section fullscreen."),
	)

	fKeyBar := RenderFKeyBar(theme, state.Width)
	dashboard = lipgloss.JoinVertical(lipgloss.Left,
		dashboard,
		fKeyBar,
	)

	return dashboard
}

func renderFocusedPanel(state *models.AppState, theme *Theme) string {
	usableWidth := state.Width
	height := state.Height - 1
	var panel *Panel

	switch state.FocusedPanel {
	case 0:
		panel = &Panel{
			Title:   "CPU / Memory Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatFocusedCPUFullContent(state, usableWidth-4),
		}
	case 1:
		panel = &Panel{
			Title:   "Storage Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatStorageContent(state.StorageMounts, state.StorageDeviceEntries, state.StorageError, state.StorageIOReadKB, state.StorageIOWriteKB, state.StorageLoopCount, false, usableWidth-6),
		}
	case 2:
		panel = &Panel{
			Title:   "Ping Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatPingContent(state.PingResults, state.PingError),
		}
	case 3:
		panel = &Panel{
			Title:  "System Detail",
			Width:  usableWidth,
			Height: height,
			Content: []string{
				"Uptime: " + formatUptime(state.Metrics.UptimeSeconds),
				"Clock: " + state.Metrics.Clock,
				fmt.Sprintf("Load: %.2f %.2f %.2f", state.Metrics.Load1, state.Metrics.Load5, state.Metrics.Load15),
				fmt.Sprintf("CPU: %.0f%%", state.Metrics.CPUPercent),
				fmt.Sprintf("Mem: %.0f%%", state.Metrics.MemoryPercent),
				fmt.Sprintf("Swap: %.0f%%", state.Metrics.SwapPercent),
			},
		}
	case 4:
		panel = &Panel{
			Title:   "Top CPU Processes",
			Width:   usableWidth,
			Height:  height,
			Content: formatTopCPUContent(state.TopCPUProcesses, state.ProcessError),
		}
	case 5:
		panel = &Panel{
			Title:   "Open Ports Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatPortsContent(state.OpenPorts, state.NetworkError),
		}
	case 6:
		panel = &Panel{
			Title:   "IP / Routes Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatIPRouteContent(state.IPRouteLines, state.NetworkError),
		}
	case 7:
		panel = &Panel{
			Title:   "Top Memory Processes",
			Width:   usableWidth,
			Height:  height,
			Content: formatTopMemContent(state.TopMemProcesses, state.ProcessError),
		}
	case 8:
		panel = &Panel{
			Title:   "Docker Containers Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatDockerContainersContent(state.DockerContainers, state.DockerError),
		}
	case 9:
		panel = &Panel{
			Title:   "Container Logs Detail",
			Width:   usableWidth,
			Height:  height,
			Content: formatContainerLogsContent(state.SelectedContainer, state.ContainerLogs, state.DockerError),
		}
	default:
		panel = &Panel{
			Title:   "Focus View",
			Width:   usableWidth,
			Height:  height,
			Content: []string{"Invalid selection"},
		}
	}

	dashboard := RenderPanel(panel, theme)
	fKeyBar := RenderFKeyBar(theme, state.Width)
	return lipgloss.JoinVertical(lipgloss.Left, dashboard, fKeyBar)
}

func formatFocusedCPUFullContent(state *models.AppState, width int) []string {
	if width < 10 {
		width = 10
	}
	return []string{
		fmt.Sprintf("CPU Usage:  %3.0f%%", state.Metrics.CPUPercent),
		renderPercentBar(state.Metrics.CPUPercent, width-4),
		"",
		fmt.Sprintf("Memory Usage:  %3.0f%%", state.Metrics.MemoryPercent),
		renderPercentBar(state.Metrics.MemoryPercent, width-4),
		"",
		fmt.Sprintf("Swap Usage:  %3.0f%%", state.Metrics.SwapPercent),
		renderPercentBar(state.Metrics.SwapPercent, width-4),
		"",
		"CPU Trend: " + renderTrendLine(state.Metrics.CPUPercent, width-12),
		"Mem Trend: " + renderTrendLine(state.Metrics.MemoryPercent, width-12),
		"Swap Trend: " + renderTrendLine(state.Metrics.SwapPercent, width-12),
		"",
		"Mem Pie:  [" + renderPieChart(state.Metrics.MemoryPercent, 12) + "]",
		"Swap Pie: [" + renderPieChart(state.Metrics.SwapPercent, 12) + "]",
		"",
		fmt.Sprintf("Load: %.2f %.2f %.2f", state.Metrics.Load1, state.Metrics.Load5, state.Metrics.Load15),
		"Back: b / Esc",
	}
}

func renderTrendLine(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	chars := []rune("▁▂▃▄▅▆▇█")
	idx := int((percent / 100) * float64(len(chars)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(chars) {
		idx = len(chars) - 1
	}
	return strings.Repeat(string(chars[idx]), width)
}

func previewTopCPUContent(processes []models.ProcessStat) []string {
	if len(processes) == 0 {
		return []string{"No process data"}
	}
	lines := make([]string, 0, 3)
	for i, p := range processes {
		if i >= 2 {
			break
		}
		lines = append(lines, fmt.Sprintf("%s %5.1f%%", p.Name, p.CPUPercent))
	}
	return lines
}

func previewTopMemContent(processes []models.ProcessStat) []string {
	if len(processes) == 0 {
		return []string{"No process data"}
	}
	lines := make([]string, 0, 3)
	for i, p := range processes {
		if i >= 2 {
			break
		}
		lines = append(lines, fmt.Sprintf("%s %5dMB", p.Name, p.MemoryMB))
	}
	return lines
}

func previewPortsContent(ports []models.PortStat) []string {
	if len(ports) == 0 {
		return []string{"No listening ports"}
	}
	lines := make([]string, 0, 3)
	for i, p := range ports {
		if i >= 2 {
			break
		}
		lines = append(lines, fmt.Sprintf("%d/%s %s", p.Port, p.Proto, p.Process))
	}
	return lines
}

func previewIPRouteContent(lines []string) []string {
	if len(lines) == 0 {
		return []string{"No network data"}
	}
	if len(lines) > 2 {
		return lines[:2]
	}
	return lines
}

func previewLogsContent(logs []string) []string {
	if len(logs) == 0 {
		return []string{"No logs"}
	}
	last := logs[len(logs)-1]
	return []string{truncateToWidth(last, 24)}
}

// Stub modal renderers for Phase 1
func renderRefreshRateModal(state *models.AppState, theme *Theme, width, height int) string {
	rates := []int{1, 3, 5, 10}
	lines := []string{
		"Select refresh interval:",
		"",
	}
	for i, rate := range rates {
		marker := "○"
		if i == state.RefreshRateModalIndex {
			marker = "●"
		}
		current := ""
		if rate == state.RefreshIntervalSec {
			current = " (current)"
		}
		lines = append(lines, fmt.Sprintf("%s %ds%s", marker, rate, current))
	}
	lines = append(lines, "", "Use ↑/↓ or 1/3/5/10, Enter to apply, Esc to cancel")
	return centerModal(theme, "Refresh Rate Settings", lines, width, height)
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

func renderPingModal(state *models.AppState, theme *Theme, width, height int) string {
	lines := make([]string, 0, len(state.PingResults)+8)
	// Source line (user-facing; keep tilde)
	lines = append(lines, "Source: ~/.config/tshoot/ping_targets.toml")
	lines = append(lines, strings.Repeat("─", max(8, width-8)))
	// If inline editing, show input fields first
	if state.PingEditing {
		if state.PingEditIndex >= 0 {
			lines = append(lines, "Editing selected target")
		} else {
			lines = append(lines, "Adding new target")
		}
		label := state.PingEditLabel
		host := state.PingEditHost
		if state.PingEditField == 0 {
			label += "_"
		} else {
			host += "_"
		}
		lines = append(lines, fmt.Sprintf("Label: %s", truncateToWidth(label, width-10)))
		lines = append(lines, fmt.Sprintf("Host : %s", truncateToWidth(host, width-10)))
		lines = append(lines, "")
		lines = append(lines, "Enter: Save   Esc: Cancel   Tab: Next field")
		lines = append(lines, "")
	}

	// Header
	lines = append(lines, fmt.Sprintf("%-18s %-20s %7s", "Label", "Host", "Status"))
	lines = append(lines, strings.Repeat("─", max(8, width-8)))

	if len(state.PingResults) == 0 {
		lines = append(lines, "No ping targets")
	} else {
		for i, p := range state.PingResults {
			prefix := " "
			if i == state.PingModalIndex {
				prefix = "●"
			} else {
				prefix = " "
			}
			status := ""
			if p.Up {
				status = fmt.Sprintf("✓ %4.0fms", p.LatencyMS)
			} else {
				status = "✗ DOWN"
			}
			label := truncateToWidth(p.Label, 18)
			host := truncateToWidth(p.Host, 20)
			lines = append(lines, fmt.Sprintf("%s %-18s %-20s %7s", prefix, label, host, status))
		}
	}

	// Spacer and action buttons
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("[ + / = / a Add ]  [ - / d Remove ]  [ Enter Edit ]  [ e Edit File ]"))
	return centerModal(theme, "📡  Ping Targets", lines, width, height)
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

// centerModal is a helper to render a centered modal
func centerModal(theme *Theme, title string, lines []string, width, height int) string {
	modalWidth := width * 2 / 5
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalWidth > width-6 {
		modalWidth = width - 6
	}

	contentLines := make([]string, 0, len(lines)+1)
	titleLine := theme.TitleStyle().Render(title)
	contentLines = append(contentLines, titleLine)
	contentLines = append(contentLines, strings.Repeat("─", modalWidth-4))

	lineStyle := lipgloss.NewStyle().Foreground(theme.TextFg).Width(modalWidth - 4)
	for _, line := range lines {
		truncated := truncateToWidth(line, modalWidth-4)
		contentLines = append(contentLines, lineStyle.Render(truncated))
	}

	content := strings.Join(contentLines, "\n")
	boxStyle := lipgloss.NewStyle().
		Border(theme.PanelBorder).
		BorderForeground(theme.PanelBorderFg).
		Background(theme.PanelBg).
		Foreground(theme.TextFg).
		Padding(1, 2).
		Width(modalWidth)

	modalBox := boxStyle.Render(content)

	boxLines := strings.Split(strings.TrimRight(modalBox, "\n"), "\n")
	topPadding := (height - len(boxLines)) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	centeredLines := make([]string, 0, topPadding+len(boxLines))
	for i := 0; i < topPadding; i++ {
		centeredLines = append(centeredLines, "")
	}

	for _, line := range boxLines {
		leftPadding := (width - lipgloss.Width(line)) / 2
		if leftPadding < 0 {
			leftPadding = 0
		}
		centeredLines = append(centeredLines, strings.Repeat(" ", leftPadding)+line)
	}

	return strings.Join(centeredLines, "\n")
}
