package agentd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

const cpuSampleWindow = 500 * time.Millisecond

type Snapshot struct {
	CPUUsage    *float64  `json:"cpu_usage"`
	MemoryUsage *float64  `json:"ram_usage"`
	DiskUsage   *float64  `json:"disk_usage"`
	Load1       *float64  `json:"load_1"`
	Load5       *float64  `json:"load_5"`
	Load15      *float64  `json:"load_15"`
	CPUCores    int       `json:"cpu_cores"`
	UptimeSec   *uint64   `json:"uptime_seconds"`
	OSInfo      string    `json:"os_info"`
	CollectedAt time.Time `json:"collected_at"`
}

type Collector interface {
	Collect(ctx context.Context) (*Snapshot, error)
}

type SystemCollector struct {
	diskPath string

	mu     sync.Mutex
	primed bool
}

func NewSystemCollector(diskPath string) *SystemCollector {
	return &SystemCollector{diskPath: diskPath}
}

func (c *SystemCollector) Collect(ctx context.Context) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s := &Snapshot{
		CPUCores:    runtime.NumCPU(),
		CollectedAt: time.Now().UTC(),
	}

	var errs []error

	if v, err := c.cpuPercent(ctx); err != nil {
		errs = append(errs, fmt.Errorf("cpu: %w", err))
	} else {
		s.CPUUsage = v
	}

	if v, err := memoryPercent(ctx); err != nil {
		errs = append(errs, fmt.Errorf("memory: %w", err))
	} else {
		s.MemoryUsage = v
	}

	if v, err := diskPercent(ctx, c.diskPath); err != nil {
		errs = append(errs, fmt.Errorf("disk %s: %w", c.diskPath, err))
	} else {
		s.DiskUsage = v
	}

	if avg, err := load.AvgWithContext(ctx); err != nil {
		errs = append(errs, fmt.Errorf("load: %w", err))
	} else {
		s.Load1 = roundLoad(avg.Load1)
		s.Load5 = roundLoad(avg.Load5)
		s.Load15 = roundLoad(avg.Load15)
	}

	if info, err := host.InfoWithContext(ctx); err != nil {
		errs = append(errs, fmt.Errorf("host: %w", err))
	} else {
		uptime := info.Uptime
		s.UptimeSec = &uptime
		s.OSInfo = formatOSInfo(info.Platform, info.PlatformVersion, info.KernelArch)
	}

	return s, errors.Join(errs...)
}

func (c *SystemCollector) cpuPercent(ctx context.Context) (*float64, error) {
	c.mu.Lock()
	window := time.Duration(0)
	if !c.primed {
		window = cpuSampleWindow
	}
	c.mu.Unlock()

	values, err := cpu.PercentWithContext(ctx, window, false)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.New("no cpu reading returned")
	}

	c.mu.Lock()
	c.primed = true
	c.mu.Unlock()

	return roundPercent(values[0]), nil
}

func memoryPercent(ctx context.Context) (*float64, error) {
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return roundPercent(stat.UsedPercent), nil
}

func diskPercent(ctx context.Context, path string) (*float64, error) {
	stat, err := disk.UsageWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	return roundPercent(stat.UsedPercent), nil
}

func formatOSInfo(platform, version, arch string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{platform, version} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}

	out := strings.Join(parts, " ")
	if arch = strings.TrimSpace(arch); arch != "" {
		if out == "" {
			return arch
		}
		out += " (" + arch + ")"
	}

	return out
}

func roundPercent(v float64) *float64 {
	return round2(math.Min(math.Max(sanitize(v), 0), 100))
}

func roundLoad(v float64) *float64 {
	return round2(math.Max(sanitize(v), 0))
}

func sanitize(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func round2(v float64) *float64 {
	r := math.Round(v*100) / 100
	return &r
}
