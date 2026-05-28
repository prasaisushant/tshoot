package collectors

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ProcessCollector tracks process CPU deltas across ticks.
type ProcessCollector struct {
	hasPrev   bool
	prevTotal uint64
	prevProc  map[int]uint64
}

// ProcessStat contains per-process cpu/memory metrics.
type ProcessStat struct {
	PID       int
	Name      string
	CPUPercent float64
	MemoryMB  int
	TotalTicks uint64
}

// NewProcessCollector creates a process collector with empty history.
func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		prevProc: make(map[int]uint64),
	}
}

// CollectTopProcesses returns top CPU and memory processes.
func CollectTopProcesses(pc *ProcessCollector, limit int) ([]ProcessStat, []ProcessStat, error) {
	if limit <= 0 {
		limit = 3
	}

	totalIdle, totalTicks, err := readCPUTotals()
	_ = totalIdle
	if err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, nil, err
	}

	stats := make([]ProcessStat, 0, 128)
	currentProcTicks := make(map[int]uint64, 256)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		name, totalProcTicks, err := readProcessStat(pid)
		if err != nil {
			continue
		}

		memMB, err := readProcessRSSMB(pid)
		if err != nil {
			memMB = 0
		}

		s := ProcessStat{
			PID:        pid,
			Name:       name,
			MemoryMB:   memMB,
			TotalTicks: totalProcTicks,
		}
		stats = append(stats, s)
		currentProcTicks[pid] = totalProcTicks
	}

	totalDelta := uint64(0)
	if pc.hasPrev && totalTicks >= pc.prevTotal {
		totalDelta = totalTicks - pc.prevTotal
	}

	for i := range stats {
		if totalDelta == 0 {
			stats[i].CPUPercent = 0
			continue
		}
		prevProcTicks := pc.prevProc[stats[i].PID]
		if stats[i].TotalTicks < prevProcTicks {
			stats[i].CPUPercent = 0
			continue
		}
		procDelta := stats[i].TotalTicks - prevProcTicks
		stats[i].CPUPercent = (float64(procDelta) / float64(totalDelta)) * 100.0
		if stats[i].CPUPercent < 0 {
			stats[i].CPUPercent = 0
		}
	}

	pc.prevTotal = totalTicks
	pc.prevProc = currentProcTicks
	pc.hasPrev = true

	topCPU := append([]ProcessStat(nil), stats...)
	sort.Slice(topCPU, func(i, j int) bool {
		return topCPU[i].CPUPercent > topCPU[j].CPUPercent
	})
	if len(topCPU) > limit {
		topCPU = topCPU[:limit]
	}

	topMem := append([]ProcessStat(nil), stats...)
	sort.Slice(topMem, func(i, j int) bool {
		if topMem[i].MemoryMB == topMem[j].MemoryMB {
			return topMem[i].PID < topMem[j].PID
		}
		return topMem[i].MemoryMB > topMem[j].MemoryMB
	})
	if len(topMem) > limit {
		topMem = topMem[:limit]
	}

	return topCPU, topMem, nil
}

func readProcessStat(pid int) (string, uint64, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	line := strings.TrimSpace(string(data))
	openIdx := strings.Index(line, "(")
	closeIdx := strings.LastIndex(line, ")")
	if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
		return "", 0, os.ErrInvalid
	}

	name := line[openIdx+1 : closeIdx]
	rest := strings.Fields(strings.TrimSpace(line[closeIdx+1:]))
	if len(rest) < 15 {
		return "", 0, os.ErrInvalid
	}

	utime, err := strconv.ParseUint(rest[11], 10, 64)
	if err != nil {
		return "", 0, err
	}
	stime, err := strconv.ParseUint(rest[12], 10, 64)
	if err != nil {
		return "", 0, err
	}

	return name, utime + stime, nil
}

func readProcessRSSMB(pid int) (int, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "status")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		rssKB, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, err
		}
		return rssKB / 1024, nil
	}

	return 0, nil
}
