package collectors

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/user/tshoot/internal/models"
)

// DockerCollector wraps a docker SDK client.
type DockerCollector struct {
	client *client.Client
}

// NewDockerCollector creates a docker collector from environment settings.
func NewDockerCollector() (*DockerCollector, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerCollector{client: c}, nil
}

// CollectContainers reads current containers for dashboard display.
func (d *DockerCollector) CollectContainers(ctx context.Context, limit int) ([]models.DockerContainerStat, error) {
	if limit <= 0 {
		limit = 6
	}
	list, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	out := make([]models.DockerContainerStat, 0, len(list))
	for _, c := range list {
		name := strings.TrimPrefix(firstOr(c.Names, c.ID), "/")
		out = append(out, models.DockerContainerStat{
			ID:     shortID(c.ID),
			Name:   name,
			Status: c.Status,
			Ports:  renderPorts(c.Ports),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// CollectContainerLogs tails logs for one container.
func (d *DockerCollector) CollectContainerLogs(ctx context.Context, containerID string, tail int) ([]string, error) {
	if containerID == "" {
		return []string{"No container selected"}, nil
	}
	if tail <= 0 {
		tail = 40
	}

	reader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Tail:       fmt.Sprintf("%d", tail),
		Follow:     false,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	lines, err := readLogLines(reader, tail)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return []string{"No logs"}, nil
	}
	return lines, nil
}

func readLogLines(r io.Reader, limit int) ([]string, error) {
	// Docker may multiplex stream headers when tty=false.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Best-effort cleaning of non-printable multiplex bytes.
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 32 && r <= 126) {
			return r
		}
		return -1
	}, string(data))

	scanner := bufio.NewScanner(strings.NewReader(cleaned))
	lines := make([]string, 0, limit)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func firstOr(items []string, fallback string) string {
	if len(items) > 0 {
		return items[0]
	}
	return fallback
}

func renderPorts(ports []container.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	chunks := make([]string, 0, len(ports))
	for _, p := range ports {
		entry := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
		if p.PublicPort > 0 {
			entry = fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type)
		}
		chunks = append(chunks, entry)
		if len(chunks) >= 2 {
			break
		}
	}
	return strings.Join(chunks, ",")
}

// ProbeDockerQuickly verifies docker daemon connectivity.
func (d *DockerCollector) ProbeDockerQuickly() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	args := filters.NewArgs()
	args.Add("status", "running")
	_, err := d.client.ContainerList(ctx, container.ListOptions{Filters: args, All: true})
	return err
}
