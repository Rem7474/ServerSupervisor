//go:build linux

package collector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type NetworkInterface struct {
	Name    string `json:"name"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

type SystemMetrics struct {
	CPUUsagePercent   float64            `json:"cpu_usage_percent"`
	CPUCores          int                `json:"cpu_cores"`
	CPUModel          string             `json:"cpu_model"`
	CPUTemperature    float64            `json:"cpu_temperature"`
	FanRPM            float64            `json:"fan_rpm"`
	LoadAvg1          float64            `json:"load_avg_1"`
	LoadAvg5          float64            `json:"load_avg_5"`
	LoadAvg15         float64            `json:"load_avg_15"`
	MemoryTotal       uint64             `json:"memory_total"`
	MemoryUsed        uint64             `json:"memory_used"`
	MemoryFree        uint64             `json:"memory_free"`
	MemoryPercent     float64            `json:"memory_percent"`
	SwapTotal         uint64             `json:"swap_total"`
	SwapUsed          uint64             `json:"swap_used"`
	Disks             []DiskInfo         `json:"disks"`
	NetworkRxBytes    uint64             `json:"network_rx_bytes"`
	NetworkTxBytes    uint64             `json:"network_tx_bytes"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces,omitempty"`
	Uptime            uint64             `json:"uptime"`
	OS                string             `json:"os"`
	Hostname          string             `json:"hostname"`
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

// CollectSystem gathers all system metrics using /proc and standard Linux tools.
// CPU temperature collection can be disabled to avoid sensor probing overhead.
func CollectSystem(collectCPUTemperature bool) (*SystemMetrics, error) {
	m := &SystemMetrics{
		CPUCores: runtime.NumCPU(),
	}

	m.Hostname, _ = os.Hostname()
	m.OS = getOSName()
	m.CPUModel = getCPUModel()
	m.CPUUsagePercent = getCPUUsage()
	if collectCPUTemperature {
		m.CPUTemperature = getCPUTemperature()
	}
	m.FanRPM = getFanRPM()

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

	rx, tx, ifaces := getNetworkBytes()
	m.NetworkRxBytes = rx
	m.NetworkTxBytes = tx
	m.NetworkInterfaces = ifaces

	m.Uptime = getUptime()

	return m, nil
}

func getCPUModel() string {
	cachedCPUModelOnce.Do(func() {
		data, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			cachedCPUModel = "unknown"
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					cachedCPUModel = strings.TrimSpace(parts[1])
					return
				}
			}
		}
		cachedCPUModel = "unknown"
	})
	return cachedCPUModel
}

func getOSName() string {
	cachedOSNameOnce.Do(func() {
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			cachedOSName = runtime.GOOS
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				value := strings.TrimPrefix(line, "PRETTY_NAME=")
				value = strings.Trim(value, "\"")
				if value != "" {
					cachedOSName = value
					return
				}
			}
		}
		cachedOSName = runtime.GOOS
	})
	return cachedOSName
}

// cachedCPUModel and cachedOSName are read once and never change at runtime.
var (
	cachedCPUModel     string
	cachedCPUModelOnce sync.Once
	cachedOSName       string
	cachedOSNameOnce   sync.Once
)

// prevCPUIdle and prevCPUTotal store the last /proc/stat sample so getCPUUsage
// can compute a delta without sleeping.
var (
	prevCPUIdle  uint64
	prevCPUTotal uint64
	cpuMu        sync.Mutex
)

func init() {
	// Store the t=0 baseline BEFORE sleeping so that main()'s first
	// CollectSystem() call measures a ~500ms window instead of ~5ms.
	// (Previously the sleep came before the store, making it a no-op.)
	idle0, total0 := readCPUStat()
	cpuMu.Lock()
	prevCPUIdle = idle0
	prevCPUTotal = total0
	cpuMu.Unlock()
	time.Sleep(500 * time.Millisecond)
}

// readCPUStat returns the idle and total CPU jiffies from /proc/stat.
func readCPUStat() (idle, total uint64) {
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

// getCPUUsage computes CPU usage as the delta between successive calls.
// The first call always returns 0 (no baseline yet). No sleep is performed.
func getCPUUsage() float64 {
	idle, total := readCPUStat()
	cpuMu.Lock()
	defer func() {
		prevCPUIdle = idle
		prevCPUTotal = total
		cpuMu.Unlock()
	}()
	if prevCPUTotal == 0 || idle < prevCPUIdle || total < prevCPUTotal {
		return 0
	}
	idleDelta := float64(idle - prevCPUIdle)
	totalDelta := float64(total - prevCPUTotal)
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

func getCPUTemperature() float64 {
	if t := readCPUTemperatureFromThermalZones(); t > 0 {
		return t
	}
	if t := readCPUTemperatureFromHwmon(); t > 0 {
		return t
	}
	if t := readCPUTemperatureFromSensors(); t > 0 {
		return t
	}
	return 0
}

func getFanRPM() float64 {
	if rpm := readFanRPMFromHwmon(); rpm > 0 {
		return rpm
	}
	if rpm := readFanRPMFromSensors(); rpm > 0 {
		return rpm
	}
	return 0
}

func readFanRPMFromHwmon() float64 {
	// fan*_input contains instantaneous fan speed in RPM.
	paths, err := filepath.Glob("/sys/class/hwmon/hwmon*/fan*_input")
	if err != nil || len(paths) == 0 {
		return 0
	}

	var sum float64
	count := 0
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil || v <= 0 {
			continue
		}
		sum += v
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func readFanRPMFromSensors() float64 {
	if _, err := exec.LookPath("sensors"); err != nil {
		return 0
	}
	out, err := exec.Command("sensors").Output()
	if err != nil {
		return 0
	}

	re := regexp.MustCompile(`(?im)fan\d+\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*RPM`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		return 0
	}

	var sum float64
	count := 0
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil || v <= 0 {
			continue
		}
		sum += v
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func readCPUTemperatureFromThermalZones() float64 {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return 0
	}

	best := 0.0
	fallback := 0.0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		base := filepath.Join("/sys/class/thermal", e.Name())
		typeName, _ := os.ReadFile(filepath.Join(base, "type"))
		raw, ok := readTempValue(filepath.Join(base, "temp"))
		if !ok {
			continue
		}
		temp := normalizeTemp(raw)
		if !validTemp(temp) {
			continue
		}

		typeLower := strings.ToLower(strings.TrimSpace(string(typeName)))
		if looksLikeCPUSensor(typeLower) {
			if temp > best {
				best = temp
			}
		} else if temp > fallback {
			fallback = temp
		}
	}

	if best > 0 {
		return best
	}
	return fallback
}

func readCPUTemperatureFromHwmon() float64 {
	entries, err := os.ReadDir("/sys/class/hwmon")
	if err != nil {
		return 0
	}

	best := 0.0
	fallback := 0.0
	for _, e := range entries {
		hwmonDir := filepath.Join("/sys/class/hwmon", e.Name())
		chipNameBytes, _ := os.ReadFile(filepath.Join(hwmonDir, "name"))
		chipName := strings.ToLower(strings.TrimSpace(string(chipNameBytes)))

		inputs, _ := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
		for _, input := range inputs {
			raw, ok := readTempValue(input)
			if !ok {
				continue
			}
			temp := normalizeTemp(raw)
			if !validTemp(temp) {
				continue
			}

			base := strings.TrimSuffix(filepath.Base(input), "_input")
			labelBytes, _ := os.ReadFile(filepath.Join(hwmonDir, base+"_label"))
			label := strings.ToLower(strings.TrimSpace(string(labelBytes)))

			if looksLikeCPUSensor(chipName) || looksLikeCPUSensor(label) {
				if temp > best {
					best = temp
				}
			} else if temp > fallback {
				fallback = temp
			}
		}
	}

	if best > 0 {
		return best
	}
	return fallback
}

var cpuTempRegex = regexp.MustCompile(`([-+]?\d+(?:\.\d+)?)\s*°C`)

func readCPUTemperatureFromSensors() float64 {
	if _, err := exec.LookPath("sensors"); err != nil {
		return 0
	}

	out, err := exec.Command("sensors").Output()
	if err != nil {
		return 0
	}

	best := 0.0
	fallback := 0.0
	for _, line := range strings.Split(string(out), "\n") {
		match := cpuTempRegex.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		temp, err := strconv.ParseFloat(match[1], 64)
		if err != nil || !validTemp(temp) {
			continue
		}

		lineLower := strings.ToLower(line)
		if looksLikeCPUSensor(lineLower) {
			if temp > best {
				best = temp
			}
		} else if temp > fallback {
			fallback = temp
		}
	}

	if best > 0 {
		return best
	}
	return fallback
}

func looksLikeCPUSensor(s string) bool {
	if s == "" {
		return false
	}
	return strings.Contains(s, "cpu") ||
		strings.Contains(s, "package") ||
		strings.Contains(s, "core") ||
		strings.Contains(s, "coretemp") ||
		strings.Contains(s, "k10temp") ||
		strings.Contains(s, "tctl") ||
		strings.Contains(s, "tdie") ||
		strings.Contains(s, "x86_pkg_temp")
}

func readTempValue(path string) (int64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func normalizeTemp(raw int64) float64 {
	if raw > 1000 {
		return float64(raw) / 1000.0
	}
	return float64(raw)
}

func validTemp(t float64) bool {
	return t >= 1 && t <= 130
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

// pseudoFS lists filesystem types that carry no real storage and must be
// excluded from disk reporting.
var pseudoFS = map[string]bool{
	"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true,
	"tmpfs": true, "cgroup": true, "cgroup2": true, "securityfs": true,
	"pstore": true, "debugfs": true, "tracefs": true, "bpf": true,
	"overlay": true, "squashfs": true, "fusectl": true, "mqueue": true,
	"hugetlbfs": true, "nsfs": true, "ramfs": true, "autofs": true,
	"binfmt_misc": true, "configfs": true, "efivarfs": true,
}

func getDiskUsage() []DiskInfo {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var disks []DiskInfo
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device, mountPoint, fsType := fields[0], fields[1], fields[2]
		if pseudoFS[fsType] || seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true

		var stat unix.Statfs_t
		if err := unix.Statfs(mountPoint, &stat); err != nil || stat.Blocks == 0 {
			continue
		}

		bsize := uint64(stat.Bsize)
		total := stat.Blocks * bsize
		free := stat.Bavail * bsize
		var used uint64
		if stat.Blocks >= stat.Bfree {
			used = (stat.Blocks - stat.Bfree) * bsize
		}
		// Compute used% the same way df does: used/(used+available).
		var pct float64
		if avail := (stat.Blocks - stat.Bfree) + stat.Bavail; avail > 0 {
			pct = float64(stat.Blocks-stat.Bfree) / float64(avail) * 100
		}

		disks = append(disks, DiskInfo{
			MountPoint:  mountPoint,
			Device:      device,
			FSType:      fsType,
			TotalBytes:  total,
			UsedBytes:   used,
			FreeBytes:   free,
			UsedPercent: pct,
		})
	}
	return disks
}

func getNetworkBytes() (rx, tx uint64, ifaces []NetworkInterface) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, nil
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
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
		ifaces = append(ifaces, NetworkInterface{Name: name, RxBytes: r, TxBytes: t})
	}
	return rx, tx, ifaces
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

// CollectMinimalMetrics gathers only essential metrics (uptime + host info).
// Used when Proxmox is the designated metrics source to track uptime changes
// (e.g., on agent restart).
func CollectMinimalMetrics() (*SystemMetrics, error) {
	m := &SystemMetrics{}
	m.Hostname, _ = os.Hostname()
	m.OS = getOSName()
	m.Uptime = getUptime()
	return m, nil
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
