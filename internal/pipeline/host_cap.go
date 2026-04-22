package pipeline

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// envMaxConcurrent reads BENDER_MAX_CONCURRENT. Non-positive or unparsable
// values return 0 so the caller falls through to the next resolution step.
func envMaxConcurrent() int {
	raw := strings.TrimSpace(os.Getenv("BENDER_MAX_CONCURRENT"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// autoCapFromHost returns a memory-aware concurrency cap, or 0 to defer to
// the caller's default. Thresholds:
//
//	available < 8 GiB  → 2
//	available < 16 GiB → 4
//	available ≥ 16 GiB → 0
//
// Best-effort: unsupported platforms and probe failures return 0.
func autoCapFromHost() int {
	avail := availableMemoryBytes()
	if avail <= 0 {
		return 0
	}
	const giB = 1 << 30
	switch {
	case avail < 8*giB:
		return 2
	case avail < 16*giB:
		return 4
	default:
		return 0
	}
}

// availableMemoryBytes returns the usable memory budget for new processes,
// or 0 when the host cannot be probed.
func availableMemoryBytes() int64 {
	switch runtime.GOOS {
	case "linux":
		return linuxMemAvailable()
	default:
		return 0
	}
}

// linuxMemAvailable parses /proc/meminfo and returns MemAvailable in bytes.
// Falls back to MemFree on older kernels that lack the MemAvailable field.
func linuxMemAvailable() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	var memAvail, memFree int64
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		switch {
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvail = parseMeminfoKB(line)
		case strings.HasPrefix(line, "MemFree:"):
			memFree = parseMeminfoKB(line)
		}
		if memAvail > 0 {
			return memAvail
		}
	}
	return memFree
}

// parseMeminfoKB extracts the kB value from a /proc/meminfo line and
// returns it converted to bytes. Returns 0 on any parse error.
func parseMeminfoKB(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n * 1024
}
