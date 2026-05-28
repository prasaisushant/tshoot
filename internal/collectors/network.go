package collectors

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prasaisushant/tshoot/internal/models"
)

// CollectOpenPorts returns listening tcp/tcp6 ports and process mappings.
func CollectOpenPorts(limit int) ([]models.PortStat, error) {
	if limit <= 0 {
		limit = 8
	}

	inodeMap := mapInodeToProcess()
	ports := make([]models.PortStat, 0, limit)

	parse := func(path, proto string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		first := true
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if first {
				first = false
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}

			// State 0A is LISTEN for TCP.
			if fields[3] != "0A" {
				continue
			}

			addrParts := strings.Split(fields[1], ":")
			if len(addrParts) != 2 {
				continue
			}
			portHex := addrParts[1]
			portU64, err := strconv.ParseUint(portHex, 16, 32)
			if err != nil {
				continue
			}

			inode := fields[9]
			pid := 0
			proc := "-"
			if info, ok := inodeMap[inode]; ok {
				pid = info.pid
				proc = info.name
			}

			ports = append(ports, models.PortStat{
				Port:    int(portU64),
				Proto:   proto,
				PID:     pid,
				Process: proc,
			})
			if len(ports) >= limit {
				return nil
			}
		}
		return scanner.Err()
	}

	_ = parse("/proc/net/tcp", "tcp")
	if len(ports) < limit {
		_ = parse("/proc/net/tcp6", "tcp6")
	}

	return ports, nil
}

type procInfo struct {
	pid  int
	name string
}

func mapInodeToProcess() map[string]procInfo {
	out := make(map[string]procInfo, 256)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		commPath := filepath.Join("/proc", e.Name(), "comm")
		commData, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}
		procName := strings.TrimSpace(string(commData))

		fdPath := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			linkTarget, err := os.Readlink(filepath.Join(fdPath, fd.Name()))
			if err != nil {
				continue
			}
			// socket:[12345]
			if !strings.HasPrefix(linkTarget, "socket:[") || !strings.HasSuffix(linkTarget, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(linkTarget, "socket:["), "]")
			if inode == "" {
				continue
			}
			if _, exists := out[inode]; !exists {
				out[inode] = procInfo{pid: pid, name: procName}
			}
		}
	}
	return out
}

// CollectIPRouteLines returns simple interface IP and route summary lines.
func CollectIPRouteLines(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 6
	}

	lines := make([]string, 0, limit)
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if v4 := ip.To4(); v4 != nil {
					lines = append(lines, iface.Name+"  "+v4.String())
					if len(lines) >= limit {
						return lines, nil
					}
				}
			}
		}
	}

	routeLines, _ := readDefaultRoutes()
	for _, r := range routeLines {
		lines = append(lines, r)
		if len(lines) >= limit {
			break
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "No network data")
	}
	return lines, nil
}

func readDefaultRoutes() ([]string, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := []string{}
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		iface := fields[0]
		destHex := fields[1]
		gwHex := fields[2]
		if destHex != "00000000" {
			continue
		}
		gw := parseHexIPv4LE(gwHex)
		out = append(out, "gw "+gw+" via "+iface)
	}

	return out, scanner.Err()
}

func parseHexIPv4LE(hexStr string) string {
	if len(hexStr) != 8 {
		return "unknown"
	}
	b1, err1 := strconv.ParseUint(hexStr[6:8], 16, 8)
	b2, err2 := strconv.ParseUint(hexStr[4:6], 16, 8)
	b3, err3 := strconv.ParseUint(hexStr[2:4], 16, 8)
	b4, err4 := strconv.ParseUint(hexStr[0:2], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return "unknown"
	}
	return strconv.FormatUint(b1, 10) + "." +
		strconv.FormatUint(b2, 10) + "." +
		strconv.FormatUint(b3, 10) + "." +
		strconv.FormatUint(b4, 10)
}
