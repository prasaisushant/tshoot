package collectors

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	if len(targets) == 0 {
		targets = []PingTarget{
			{Label: "Google DNS", Host: "8.8.8.8"},
			{Label: "Cloudflare", Host: "1.1.1.1"},
		}
	}
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
