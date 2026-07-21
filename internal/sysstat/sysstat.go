// Package sysstat samples host and per-process OS resource usage (CPU, memory,
// disk) via gopsutil. It is a read-only side channel with no gin/gorm
// dependency: the API layer owns an instance and calls Host / Process on each
// polling request. CPU percentages are computed from the delta between
// successive samples, so the collector holds a small amount of state (the last
// CPU baseline per key) guarded by a mutex.
package sysstat

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// baselineExpiry is how stale a CPU baseline may be before it is discarded and
// the next sample is treated as a first frame (returning 0). A polling client
// refreshes every ~5s; a gap longer than this means the delta would be
// meaningless (page was backgrounded, tab reopened, etc.).
const baselineExpiry = 30 * time.Second

// HostStats is the whole-machine resource snapshot returned by Host.
type HostStats struct {
	CPUPercent  float64 `json:"cpuPercent"` // 0..100, normalized across cores
	NumCPU      int     `json:"numCpu"`
	MemUsed     uint64  `json:"memUsed"` // bytes
	MemTotal    uint64  `json:"memTotal"`
	MemPercent  float64 `json:"memPercent"`
	DiskUsed    uint64  `json:"diskUsed"`
	DiskTotal   uint64  `json:"diskTotal"`
	DiskPercent float64 `json:"diskPercent"`
}

// ProcessStats is the per-server process-tree resource snapshot returned by
// Process. CPUPercent is per-core (may exceed 100 on multi-core loads).
type ProcessStats struct {
	Running      bool    `json:"running"`
	Reason       string  `json:"reason,omitempty"`
	PID          int     `json:"pid,omitempty"`
	CPUPercent   float64 `json:"cpuPercent"`
	NumCPU       int     `json:"numCpu"`
	MemoryRSS    uint64  `json:"memoryRss"` // bytes, summed over the tree
	ProcessCount int     `json:"processCount"`
}

// cpuSample is the CPU baseline stored per key. totalCPUSeconds is the summed
// User+System CPU time of a process tree at wall time at.
type cpuSample struct {
	totalCPUSeconds float64
	at              time.Time
}

// Collector samples OS resource usage and holds the per-key CPU baseline used
// to compute process-tree CPU percentages between calls. It is safe for
// concurrent use (multiple browser tabs may poll simultaneously).
type Collector struct {
	mu      sync.Mutex
	samples map[string]cpuSample
	numCPU  int
}

// New creates a Collector. numCPU is resolved once via gopsutil (logical core
// count); it is used to normalize host CPU and reported to the frontend.
func New() *Collector {
	n, err := cpu.Counts(true)
	if err != nil || n <= 0 {
		n = 1
	}
	return &Collector{
		samples: make(map[string]cpuSample),
		numCPU:  n,
	}
}

// NumCPU returns the logical core count resolved at construction. Used by the
// API layer to populate the not-running ProcessStats response.
func (c *Collector) NumCPU() int { return c.numCPU }

// Host returns a whole-machine snapshot. Each metric group (CPU, memory, disk)
// is sampled independently: a failure in one leaves its fields at zero and does
// not abort the others, so the endpoint always returns a usable 200.
func (c *Collector) Host(ctx context.Context) HostStats {
	s := HostStats{NumCPU: c.numCPU}

	// cpu.Percent(0, false) returns the aggregate across all cores since the
	// last call; gopsutil maintains its own global baseline, so the first call
	// in the process may return 0 and the next poll corrects it.
	if pcts, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pcts) > 0 {
		s.CPUPercent = pcts[0]
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
		s.MemUsed = vm.Used
		s.MemTotal = vm.Total
		s.MemPercent = vm.UsedPercent
	}

	// Report the volume that holds the working directory (the app's data disk).
	if du, err := disk.UsageWithContext(ctx, "."); err == nil && du != nil {
		s.DiskUsed = du.Used
		s.DiskTotal = du.Total
		s.DiskPercent = du.UsedPercent
	}

	return s
}

// Process returns a snapshot for the process tree rooted at pid, aggregating
// memory (RSS) and CPU across the root and all descendants. key namespaces the
// CPU baseline (typically the server id). If the root process is gone it
// returns a not-running result with a nil-equivalent (no error) so callers can
// respond structurally rather than 500.
func (c *Collector) Process(ctx context.Context, key string, pid int) ProcessStats {
	stats := ProcessStats{PID: pid, NumCPU: c.numCPU}

	root, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		stats.Running = false
		stats.Reason = "not_running"
		return stats
	}

	rss, cpuSeconds, count := gatherTree(ctx, root)
	if count == 0 {
		// Root vanished between NewProcess and the walk.
		stats.Running = false
		stats.Reason = "not_running"
		return stats
	}

	stats.Running = true
	stats.MemoryRSS = rss
	stats.ProcessCount = count
	stats.CPUPercent = c.cpuDelta(key, cpuSeconds)
	return stats
}

// gatherTree walks the process subtree rooted at root, summing RSS and CPU
// seconds (User+System) and counting reachable nodes. A node that errors mid
// walk (just exited) is skipped rather than failing the whole aggregation.
func gatherTree(ctx context.Context, root *process.Process) (rss uint64, cpuSeconds float64, count int) {
	var visit func(p *process.Process)
	seen := make(map[int32]struct{})
	visit = func(p *process.Process) {
		if p == nil {
			return
		}
		if _, ok := seen[p.Pid]; ok {
			return
		}
		seen[p.Pid] = struct{}{}
		count++

		if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
			rss += mi.RSS
		}
		if t, err := p.TimesWithContext(ctx); err == nil && t != nil {
			cpuSeconds += t.User + t.System
		}
		if children, err := p.ChildrenWithContext(ctx); err == nil {
			for _, child := range children {
				visit(child)
			}
		}
	}
	visit(root)
	return rss, cpuSeconds, count
}

// cpuDelta converts an absolute total-CPU-seconds reading into a percentage
// using the stored baseline for key. It updates the baseline and returns 0 on
// the first frame or when the previous baseline is missing/expired. The
// per-core semantics are preserved (no division by numCPU): a fully busy
// single-threaded process reads ~100, a busy quad-threaded one ~400.
func (c *Collector) cpuDelta(key string, totalCPUSeconds float64) float64 {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	prev, ok := c.samples[key]
	c.samples[key] = cpuSample{totalCPUSeconds: totalCPUSeconds, at: now}

	if !ok || now.Sub(prev.at) > baselineExpiry {
		return 0
	}
	wall := now.Sub(prev.at).Seconds()
	if wall <= 0 {
		return 0
	}
	deltaCPU := totalCPUSeconds - prev.totalCPUSeconds
	if deltaCPU < 0 {
		deltaCPU = 0
	}
	return deltaCPU / wall * 100
}

