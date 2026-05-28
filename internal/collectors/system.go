package collectors

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Snapshot contains one read of system metrics from Linux procfs.
type Snapshot struct {
	CPUPercent    float64
	MemoryPercent float64
	SwapPercent   float64
	MemoryUsedMB  int
	MemoryTotalMB int
	SwapUsedMB    int
	SwapTotalMB   int
	Load1         float64
	Load5         float64
	Load15        float64
	UptimeSeconds float64
	Clock         string
}

// CPUCalculator tracks previous cpu counters to compute utilization delta.
type CPUCalculator struct {
	hasPrev   bool
	prevIdle  uint64
	prevTotal uint64
}

// CollectSnapshot reads procfs values and computes a usable dashboard snapshot.
func CollectSnapshot(cpu *CPUCalculator) (Snapshot, error) {
	s := Snapshot{
		Clock: time.Now().Format("15:04:05"),
	}

	var errs []string

	if pct, err := readCPUPercent(cpu); err == nil {
		s.CPUPercent = pct
	} else {
		errs = append(errs, "cpu")
	}

	if mem, err := readMemInfo(); err == nil {
		s.MemoryPercent = mem.memoryPercent
		s.SwapPercent = mem.swapPercent
		s.MemoryUsedMB = mem.memoryUsedMB
		s.MemoryTotalMB = mem.memoryTotalMB
		s.SwapUsedMB = mem.swapUsedMB
		s.SwapTotalMB = mem.swapTotalMB
	} else {
		errs = append(errs, "memory")
	}

	if l1, l5, l15, err := readLoadAvg(); err == nil {
		s.Load1 = l1
		s.Load5 = l5
		s.Load15 = l15
	} else {
		errs = append(errs, "load")
	}

	if uptime, err := readUptime(); err == nil {
		s.UptimeSeconds = uptime
	} else {
		errs = append(errs, "uptime")
	}

	if len(errs) > 0 {
		return s, errors.New(strings.Join(errs, ","))
	}
	return s, nil
}

func readCPUPercent(cpu *CPUCalculator) (float64, error) {
	idle, total, err := readCPUTotals()
	if err != nil {
		return 0, err
	}

	if !cpu.hasPrev {
		cpu.prevIdle = idle
		cpu.prevTotal = total
		cpu.hasPrev = true
		return 0, nil
	}

	idleDelta := idle - cpu.prevIdle
	totalDelta := total - cpu.prevTotal
	cpu.prevIdle = idle
	cpu.prevTotal = total

	if totalDelta == 0 {
		return 0, nil
	}

	usage := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, nil
}

func readCPUTotals() (uint64, uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, errors.New("empty /proc/stat")
	}
	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errors.New("invalid cpu line")
	}

	var total uint64
	var vals []uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		vals = append(vals, v)
		total += v
	}

	idle := vals[3]
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}

	return idle, total, nil
}

type memInfo struct {
	memoryPercent float64
	swapPercent   float64
	memoryUsedMB  int
	memoryTotalMB int
	swapUsedMB    int
	swapTotalMB   int
}

func readMemInfo() (memInfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return memInfo{}, err
	}
	defer f.Close()

	valuesKB := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		v, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		valuesKB[key] = v
	}

	memTotal := valuesKB["MemTotal"]
	memAvailable := valuesKB["MemAvailable"]
	swapTotal := valuesKB["SwapTotal"]
	swapFree := valuesKB["SwapFree"]
	if memTotal == 0 {
		return memInfo{}, errors.New("missing MemTotal")
	}

	memUsed := memTotal - memAvailable
	memoryPercent := float64(memUsed) / float64(memTotal) * 100

	var swapUsed uint64
	var swapPercent float64
	if swapTotal > 0 {
		swapUsed = swapTotal - swapFree
		swapPercent = float64(swapUsed) / float64(swapTotal) * 100
	}

	return memInfo{
		memoryPercent: memoryPercent,
		swapPercent:   swapPercent,
		memoryUsedMB:  int(memUsed / 1024),
		memoryTotalMB: int(memTotal / 1024),
		swapUsedMB:    int(swapUsed / 1024),
		swapTotalMB:   int(swapTotal / 1024),
	}, nil
}

func readLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, errors.New("invalid /proc/loadavg")
	}
	l1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	l5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	l15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return l1, l5, l15, nil
}

func readUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, errors.New("invalid /proc/uptime")
	}
	return strconv.ParseFloat(fields[0], 64)
}
