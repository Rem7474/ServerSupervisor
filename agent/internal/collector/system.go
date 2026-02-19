package collector

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type SystemMetrics struct {
	CPUUsagePercent float64    `json:"cpu_usage_percent"`
	CPUCores        int        `json:"cpu_cores"`
	CPUModel        string     `json:"cpu_model"`
	LoadAvg1        float64    `json:"load_avg_1"`
	LoadAvg5        float64    `json:"load_avg_5"`
	LoadAvg15       float64    `json:"load_avg_15"`
	MemoryTotal     uint64     `json:"memory_total"`
	MemoryUsed      uint64     `json:"memory_used"`
	MemoryFree      uint64     `json:"memory_free"`
	MemoryPercent   float64    `json:"memory_percent"`
	SwapTotal       uint64     `json:"swap_total"`
	SwapUsed        uint64     `json:"swap_used"`
	Disks           []DiskInfo `json:"disks"`
	NetworkRxBytes  uint64     `json:"network_rx_bytes"`
	NetworkTxBytes  uint64     `json:"network_tx_bytes"`
	Uptime          uint64     `json:"uptime"`
	OS              string     `json:"os"`
	Hostname        string     `json:"hostname"`
}

type DiskInfo struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	FSType      string  `json:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// CollectSystem gathers all system metrics using /proc and standard Linux tools
func CollectSystem() (*SystemMetrics, error) {
	m := &SystemMetrics{
		CPUCores: runtime.NumCPU(),
	}

	m.Hostname, _ = os.Hostname()
	m.OS = getOSName()
	m.CPUModel = getCPUModel()
	m.CPUUsagePercent = getCPUUsage()

	loadAvg := getLoadAvg()
	if len(loadAvg) == 3 {
		m.LoadAvg1 = loadAvg[0]
		m.LoadAvg5 = loadAvg[1]
		m.LoadAvg15 = loadAvg[2]
	}

	memInfo := getMemInfo()
	m.MemoryTotal = memInfo["MemTotal"]
	m.MemoryFree = memInfo["MemAvailable"]
	m.MemoryUsed = m.MemoryTotal - m.MemoryFree
	if m.MemoryTotal > 0 {
		m.MemoryPercent = float64(m.MemoryUsed) / float64(m.MemoryTotal) * 100
	}
	m.SwapTotal = memInfo["SwapTotal"]
	m.SwapUsed = m.SwapTotal - memInfo["SwapFree"]

	m.Disks = getDiskUsage()

	rx, tx := getNetworkBytes()
	m.NetworkRxBytes = rx
	m.NetworkTxBytes = tx

	m.Uptime = getUptime()

	return m, nil
}

func getCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "unknown"
}

func getOSName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			value := strings.TrimPrefix(line, "PRETTY_NAME=")
			value = strings.Trim(value, "\"")
			if value != "" {
				return value
			}
		}
	}
	return runtime.GOOS
}

func getCPUUsage() float64 {
	// Read /proc/stat twice with a short interval
	read := func() (idle, total uint64) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			return 0, 0
		}
		fields := strings.Fields(lines[0])
		if len(fields) < 5 {
			return 0, 0
		}
		var values []uint64
		for _, f := range fields[1:] {
			v, _ := strconv.ParseUint(f, 10, 64)
			values = append(values, v)
		}
		var sum uint64
		for _, v := range values {
			sum += v
		}
		if len(values) >= 4 {
			return values[3], sum
		}
		return 0, sum
	}

	idle1, total1 := read()
	time.Sleep(500 * time.Millisecond)
	idle2, total2 := read()

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta == 0 {
		return 0
	}
	return (1.0 - idleDelta/totalDelta) * 100
}

func getLoadAvg() []float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return []float64{0, 0, 0}
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return []float64{0, 0, 0}
	}
	result := make([]float64, 3)
	for i := 0; i < 3; i++ {
		result[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return result
}

func getMemInfo() map[string]uint64 {
	result := make(map[string]uint64)
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		val, _ := strconv.ParseUint(strings.TrimSpace(valStr), 10, 64)
		result[key] = val * 1024 // Convert to bytes
	}
	return result
}

func getDiskUsage() []DiskInfo {
	out, err := exec.Command("df", "-B1", "--output=target,source,fstype,size,used,avail,pcent").Output()
	if err != nil {
		return nil
	}

	var disks []DiskInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		// Skip pseudo-filesystems
		if strings.HasPrefix(fields[1], "tmpfs") || strings.HasPrefix(fields[1], "devtmpfs") ||
			fields[0] == "/boot/efi" || strings.HasPrefix(fields[2], "squashfs") {
			continue
		}

		total, _ := strconv.ParseUint(fields[3], 10, 64)
		used, _ := strconv.ParseUint(fields[4], 10, 64)
		free, _ := strconv.ParseUint(fields[5], 10, 64)
		pctStr := strings.TrimSuffix(fields[6], "%")
		pct, _ := strconv.ParseFloat(pctStr, 64)

		disks = append(disks, DiskInfo{
			MountPoint:  fields[0],
			Device:      fields[1],
			FSType:      fields[2],
			TotalBytes:  total,
			UsedBytes:   used,
			FreeBytes:   free,
			UsedPercent: pct,
		})
	}
	return disks
}

func getNetworkBytes() (rx, tx uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "lo:") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		_ = iface
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx
}

func getUptime() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	val, _ := strconv.ParseFloat(fields[0], 64)
	return uint64(val)
}

func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
