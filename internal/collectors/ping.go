package collectors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/prasaisushant/tshoot/internal/models"
)

// PingTarget is one configured ping endpoint.
type PingTarget struct {
	Label string
	Host  string
}

var latencyRegex = regexp.MustCompile(`time=([0-9.]+)\s*ms`)

// CollectPingStatuses checks each target via system ping command.
func CollectPingStatuses(targets []PingTarget, timeout time.Duration) ([]models.PingStat, error) {
	targets = EnsureDefaultPingTargets(targets)
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}

	results := make([]models.PingStat, 0, len(targets))
	var lastErr error

	for _, t := range targets {
		up, latency, err := pingOne(t.Host, timeout)
		if err != nil {
			lastErr = err
		}
		results = append(results, models.PingStat{
			Label:     t.Label,
			Host:      t.Host,
			Up:        up,
			LatencyMS: latency,
		})
	}

	return results, lastErr
}

func pingOne(host string, timeout time.Duration) (bool, float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+400*time.Millisecond)
	defer cancel()

	secs := int(timeout / time.Second)
	if secs < 1 {
		secs = 1
	}

	// Linux ping flags: -c count, -W timeout seconds.
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(secs), host)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return false, 0, err
	}

	m := latencyRegex.FindStringSubmatch(output)
	if len(m) < 2 {
		// command succeeded but no parsable latency
		return true, 0, nil
	}

	v, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
	if err != nil {
		return true, 0, nil
	}
	return true, v, nil
}

// pingConfig is the on-disk TOML structure
type pingConfig struct {
	Targets []PingTarget `toml:"targets"`
}

var defaultPingTargets = []PingTarget{
	{Label: "Google DNS", Host: "8.8.8.8"},
	{Label: "Cloudflare", Host: "1.1.1.1"},
}

// DefaultPingTargets returns the default ping endpoints.
func DefaultPingTargets() []PingTarget {
	targets := make([]PingTarget, len(defaultPingTargets))
	copy(targets, defaultPingTargets)
	return targets
}

func targetExists(targets []PingTarget, target PingTarget) bool {
	for _, t := range targets {
		if t.Host == target.Host {
			return true
		}
	}
	return false
}

func EnsureDefaultPingTargets(targets []PingTarget) []PingTarget {
	if len(targets) == 0 {
		return DefaultPingTargets()
	}

	merged := make([]PingTarget, 0, len(targets)+len(defaultPingTargets))
	merged = append(merged, targets...)
	for _, def := range defaultPingTargets {
		if !targetExists(merged, def) {
			merged = append(merged, def)
		}
	}
	return merged
}

func IsDefaultPingTarget(target PingTarget) bool {
	for _, def := range defaultPingTargets {
		if target.Host == def.Host {
			return true
		}
	}
	return false
}

// LoadPingTargetsFromFile loads ping targets from a TOML file at path.
// Expected format:
// [[targets]]
// label = "Google DNS"
// host = "8.8.8.8"
func LoadPingTargetsFromFile(path string) ([]PingTarget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg pingConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, err
	}
	return cfg.Targets, nil
}

// SavePingTargetsToFile writes the targets to the given TOML path, creating parent dirs as needed.
func SavePingTargetsToFile(path string, targets []PingTarget) error {
	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg := pingConfig{Targets: targets}
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	return nil
}
