package collectors

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prasaisushant/tshoot/internal/models"
)

const sectorSizeBytes = 512

// DiskIOCollector computes read/write rates from /proc/diskstats.
type DiskIOCollector struct {
	hasPrev   bool
	prevStats map[string]diskStat
	prevTime  time.Time
}

type diskStat struct {
	readSectors  uint64
	writeSectors uint64
}

// NewDiskIOCollector creates a collector for disk I/O metrics.
func NewDiskIOCollector() *DiskIOCollector {
	return &DiskIOCollector{
		prevStats: make(map[string]diskStat),
	}
}

// CollectStorageSummary returns parsed mount and device entries plus loop device count.
func CollectStorageSummary(limit int) ([]models.StorageMount, []models.StorageDeviceEntry, int, error) {
	if limit <= 0 {
		limit = 4
	}

	mounts, mountErr, loopCount := collectDFMounts(limit)
	devices, deviceErr, loopCount2 := collectLSBLKDevices(limit)
	loopCount += loopCount2

	if mountErr != nil && deviceErr != nil {
		return nil, nil, loopCount, fmt.Errorf("df: %w; lsblk: %v", mountErr, deviceErr)
	}
	if mountErr != nil {
		return mounts, devices, loopCount, fmt.Errorf("df: %w", mountErr)
	}
	if deviceErr != nil {
		return mounts, devices, loopCount, fmt.Errorf("lsblk: %w", deviceErr)
	}
	return mounts, devices, loopCount, nil
}

func collectDFMounts(limit int) ([]models.StorageMount, error, int) {
	cmd := exec.Command("df", "-Th")
	output, err := cmd.Output()
	if err != nil {
		return nil, err, 0
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	mounts := make([]models.StorageMount, 0, limit)
	loopCount := 0
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 7 {
			continue
		}

		device := fields[0]
		fstype := fields[1]
		size := fields[2]
		usePct := fields[5]
		mount := strings.Join(fields[6:], " ")

		if strings.HasPrefix(device, "tmpfs") || strings.HasPrefix(device, "udev") {
			continue
		}
		if strings.HasPrefix(device, "/dev/loop") || strings.HasPrefix(device, "loop") {
			loopCount++
			continue
		}

		mounts = append(mounts, models.StorageMount{
			Mount:  mount,
			FSType: fstype,
			UsePct: usePct,
			Size:   size,
		})
		if len(mounts) >= limit {
			break
		}
	}

	if len(mounts) == 0 {
		return []models.StorageMount{}, nil, loopCount
	}
	return mounts, scanner.Err(), loopCount
}

func collectLSBLKDevices(limit int) ([]models.StorageDeviceEntry, error, int) {
	cmd := exec.Command("lsblk", "-ndo", "NAME,SIZE,TYPE,MOUNTPOINT")
	output, err := cmd.Output()
	if err != nil {
		return nil, err, 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	devices := make([]models.StorageDeviceEntry, 0, limit)
	loopCount := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		size := fields[1]
		typeName := fields[2]
		mount := ""
		if len(fields) > 3 {
			mount = strings.Join(fields[3:], " ")
		}

		if strings.HasPrefix(name, "loop") {
			loopCount++
			continue
		}

		devices = append(devices, models.StorageDeviceEntry{
			Name:  name,
			Size:  size,
			Type:  typeName,
			Mount: mount,
		})
		if len(devices) >= limit {
			break
		}
	}

	if len(devices) == 0 {
		return []models.StorageDeviceEntry{}, nil, loopCount
	}
	return devices, nil, loopCount
}

// CollectIOSpeeds returns read and write rates in KB/s for non-loop block devices.
func (c *DiskIOCollector) CollectIOSpeeds() (float64, float64, error) {
	stats, now, err := readDiskStats()
	if err != nil {
		return 0, 0, err
	}
	if !c.hasPrev {
		c.prevStats = stats
		c.prevTime = now
		c.hasPrev = true
		return 0, 0, nil
	}

	deltaSeconds := now.Sub(c.prevTime).Seconds()
	if deltaSeconds <= 0 {
		deltaSeconds = 1
	}

	readBytes := uint64(0)
	writeBytes := uint64(0)
	for name, current := range stats {
		if shouldIgnoreDisk(name) {
			continue
		}
		prev, ok := c.prevStats[name]
		if !ok {
			continue
		}
		if current.readSectors >= prev.readSectors {
			readBytes += (current.readSectors - prev.readSectors) * sectorSizeBytes
		}
		if current.writeSectors >= prev.writeSectors {
			writeBytes += (current.writeSectors - prev.writeSectors) * sectorSizeBytes
		}
	}

	c.prevStats = stats
	c.prevTime = now
	return float64(readBytes) / 1024.0 / deltaSeconds, float64(writeBytes) / 1024.0 / deltaSeconds, nil
}

func readDiskStats() (map[string]diskStat, time.Time, error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, time.Time{}, err
	}

	stats := make(map[string]diskStat)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		readSectors, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			continue
		}
		writeSectors, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		stats[name] = diskStat{readSectors: readSectors, writeSectors: writeSectors}
	}
	return stats, time.Now(), scanner.Err()
}

func shouldIgnoreDisk(name string) bool {
	return strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") || strings.HasPrefix(name, "fd")
}
