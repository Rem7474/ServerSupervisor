package collector

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

// DiskMetrics contient les informations détaillées sur l'utilisation du disque
type DiskMetrics struct {
	MountPoint    string  `json:"mount_point"`
	Filesystem    string  `json:"filesystem"`
	SizeGB        float64 `json:"size_gb"`
	UsedGB        float64 `json:"used_gb"`
	AvailGB       float64 `json:"avail_gb"`
	UsedPercent   float64 `json:"used_percent"`
	InodesTotal   int64   `json:"inodes_total"`
	InodesUsed    int64   `json:"inodes_used"`
	InodesFree    int64   `json:"inodes_free"`
	InodesPercent float64 `json:"inodes_percent"`
}

// DiskHealth contient les informations SMART sur la santé du disque
type DiskHealth struct {
	Device               string `json:"device"`
	Model                string `json:"model"`
	SerialNumber         string `json:"serial_number"`
	SMARTStatus          string `json:"smart_status"` // PASSED, FAILED, UNKNOWN
	Temperature          int    `json:"temperature"`
	PowerOnHours         int    `json:"power_on_hours"`
	ReallocatedSectors   int    `json:"reallocated_sectors"`
	PendingSectors       int    `json:"pending_sectors"`
	UncorrectableSectors int    `json:"uncorrectable_sectors"`
	PercentageUsed       int    `json:"percentage_used"` // For SSDs
}

// CollectDiskMetrics collecte les informations détaillées sur tous les systèmes de fichiers montés
func CollectDiskMetrics() ([]DiskMetrics, error) {
	// Exécuter df -BG pour obtenir les tailles en GB
	cmdSpace := exec.Command("df", "-BG", "--output=source,target,size,used,avail,pcent,fstype")
	outSpace, err := cmdSpace.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// Exécuter df -i pour obtenir les informations sur les inodes
	cmdInodes := exec.Command("df", "-i", "--output=source,target,itotal,iused,iavail,ipcent")
	outInodes, err := cmdInodes.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// Parser les résultats
	spaceMap := parseDfSpace(string(outSpace))
	inodesMap := parseDfInodes(string(outInodes))

	// Fusionner les résultats
	var metrics []DiskMetrics
	for mountPoint, space := range spaceMap {
		metric := space
		if inode, ok := inodesMap[mountPoint]; ok {
			metric.InodesTotal = inode.InodesTotal
			metric.InodesUsed = inode.InodesUsed
			metric.InodesFree = inode.InodesFree
			metric.InodesPercent = inode.InodesPercent
		}
		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func parseDfSpace(output string) map[string]DiskMetrics {
	result := make(map[string]DiskMetrics)
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 || line == "" {
			continue // Skip header and empty lines
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// Skip pseudo-filesystems
		fstype := fields[6]
		if fstype == "tmpfs" || fstype == "devtmpfs" || fstype == "squashfs" ||
			fstype == "overlay" || fstype == "devfs" {
			continue
		}

		mountPoint := fields[1]

		// Parse sizes (remove 'G' suffix)
		sizeStr := strings.TrimSuffix(fields[2], "G")
		usedStr := strings.TrimSuffix(fields[3], "G")
		availStr := strings.TrimSuffix(fields[4], "G")
		pctStr := strings.TrimSuffix(fields[5], "%")

		size, _ := strconv.ParseFloat(sizeStr, 64)
		used, _ := strconv.ParseFloat(usedStr, 64)
		avail, _ := strconv.ParseFloat(availStr, 64)
		pct, _ := strconv.ParseFloat(pctStr, 64)

		result[mountPoint] = DiskMetrics{
			MountPoint:  mountPoint,
			Filesystem:  fields[0],
			SizeGB:      size,
			UsedGB:      used,
			AvailGB:     avail,
			UsedPercent: pct,
		}
	}

	return result
}

func parseDfInodes(output string) map[string]DiskMetrics {
	result := make(map[string]DiskMetrics)
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		mountPoint := fields[1]

		total, _ := strconv.ParseInt(fields[2], 10, 64)
		used, _ := strconv.ParseInt(fields[3], 10, 64)
		free, _ := strconv.ParseInt(fields[4], 10, 64)
		pctStr := strings.TrimSuffix(fields[5], "%")
		pct, _ := strconv.ParseFloat(pctStr, 64)

		result[mountPoint] = DiskMetrics{
			MountPoint:    mountPoint,
			InodesTotal:   total,
			InodesUsed:    used,
			InodesFree:    free,
			InodesPercent: pct,
		}
	}

	return result
}

// CollectDiskHealth collecte les informations SMART sur tous les disques physiques
// Nécessite smartmontools installé (smartctl)
func CollectDiskHealth() ([]DiskHealth, error) {
	// Trouver tous les disques physiques
	devices, err := findPhysicalDisks()
	if err != nil {
		return nil, err
	}

	var healthData []DiskHealth
	for _, device := range devices {
		health, err := collectSmartData(device)
		if err != nil {
			// Si smartctl échoue, on continue avec les autres disques
			continue
		}
		healthData = append(healthData, health)
	}

	return healthData, nil
}

// findPhysicalDisks trouve tous les disques physiques (sd*, nvme*, vd*)
func findPhysicalDisks() ([]string, error) {
	cmd := exec.Command("sh", "-c", "ls /dev/sd[a-z] /dev/nvme[0-9]n[0-9] /dev/vd[a-z] 2>/dev/null | sort -u")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Pas de disques trouvés ou commande échouée
		return []string{}, nil
	}

	devices := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			devices = append(devices, line)
		}
	}

	return devices, nil
}

// collectSmartData récupère les données SMART pour un disque spécifique
func collectSmartData(device string) (DiskHealth, error) {
	health := DiskHealth{
		Device:      device,
		SMARTStatus: "UNKNOWN",
	}

	// Exécuter smartctl avec sortie JSON (disponible depuis smartmontools 7.0)
	cmd := exec.Command("smartctl", "-A", "-i", "-H", "-j", device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// smartctl retourne un code d'erreur non-zéro si SMART n'est pas supporté
		// On essaie quand même de parser la sortie
	}

	// Parser la sortie JSON
	var smartData map[string]interface{}
	if err := json.Unmarshal(out, &smartData); err != nil {
		// Si JSON échoue, fallback sur parsing texte
		return parseSmartText(device, string(out))
	}

	// Extraire les informations du JSON
	if info, ok := smartData["model_name"].(string); ok {
		health.Model = info
	}
	if serial, ok := smartData["serial_number"].(string); ok {
		health.SerialNumber = serial
	}

	// SMART status
	if status, ok := smartData["smart_status"].(map[string]interface{}); ok {
		if passed, ok := status["passed"].(bool); ok {
			if passed {
				health.SMARTStatus = "PASSED"
			} else {
				health.SMARTStatus = "FAILED"
			}
		}
	}

	// Temperature
	if temp, ok := smartData["temperature"].(map[string]interface{}); ok {
		if current, ok := temp["current"].(float64); ok {
			health.Temperature = int(current)
		}
	}

	// Attributes SMART
	if attrs, ok := smartData["ata_smart_attributes"].(map[string]interface{}); ok {
		if table, ok := attrs["table"].([]interface{}); ok {
			for _, attr := range table {
				if attrMap, ok := attr.(map[string]interface{}); ok {
					id := int(attrMap["id"].(float64))
					rawValue := int64(attrMap["raw"].(map[string]interface{})["value"].(float64))

					switch id {
					case 5: // Reallocated Sectors Count
						health.ReallocatedSectors = int(rawValue)
					case 9: // Power On Hours
						health.PowerOnHours = int(rawValue)
					case 197: // Current Pending Sector Count
						health.PendingSectors = int(rawValue)
					case 198: // Offline Uncorrectable Sector Count
						health.UncorrectableSectors = int(rawValue)
					}
				}
			}
		}
	}

	// Pour les NVMe SSD
	if nvme, ok := smartData["nvme_smart_health_information_log"].(map[string]interface{}); ok {
		if temp, ok := nvme["temperature"].(float64); ok {
			health.Temperature = int(temp)
		}
		if pct, ok := nvme["percentage_used"].(float64); ok {
			health.PercentageUsed = int(pct)
		}
		if hours, ok := nvme["power_on_hours"].(float64); ok {
			health.PowerOnHours = int(hours)
		}
	}

	return health, nil
}

// parseSmartText fallback pour parser la sortie texte de smartctl
func parseSmartText(device, output string) (DiskHealth, error) {
	health := DiskHealth{
		Device:      device,
		SMARTStatus: "UNKNOWN",
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				health.Model = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Serial Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				health.SerialNumber = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "SMART overall-health") {
			if strings.Contains(line, "PASSED") {
				health.SMARTStatus = "PASSED"
			} else if strings.Contains(line, "FAILED") {
				health.SMARTStatus = "FAILED"
			}
		} else if strings.HasPrefix(line, "Temperature:") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "Temperature:" && i+1 < len(fields) {
					temp, _ := strconv.Atoi(fields[i+1])
					health.Temperature = temp
					break
				}
			}
		}

		// Parser les attributs SMART (format: ID# NAME FLAG VALUE WORST THRESH TYPE UPDATED WHEN_FAILED RAW_VALUE)
		fields := strings.Fields(line)
		if len(fields) >= 10 && len(fields[0]) > 0 && fields[0][0] >= '0' && fields[0][0] <= '9' {
			id, _ := strconv.Atoi(fields[0])
			rawValue, _ := strconv.ParseInt(fields[9], 10, 64)

			switch id {
			case 5:
				health.ReallocatedSectors = int(rawValue)
			case 9:
				health.PowerOnHours = int(rawValue)
			case 197:
				health.PendingSectors = int(rawValue)
			case 198:
				health.UncorrectableSectors = int(rawValue)
			}
		}
	}

	return health, nil
}
