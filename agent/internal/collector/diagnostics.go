package collector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/serversupervisor/agent/internal/config"
)

// DiagnosticSeverity distinguishes a collector that will produce zero data
// (Error) from one that is only partially degraded (Warning).
type DiagnosticSeverity string

const (
	DiagnosticError   DiagnosticSeverity = "error"
	DiagnosticWarning DiagnosticSeverity = "warning"
)

// DiagnosticIssue is one config-vs-reality mismatch found by CheckConfig —
// e.g. collect_restic: true with a resticconf path that doesn't exist. Sent
// to the server as part of the periodic report (see sender.Report.Diagnostics)
// so it can be surfaced in the UI without waiting for the feature to actually
// fail at runtime.
type DiagnosticIssue struct {
	Collector string             `json:"collector"` // e.g. "restic", "web_logs", "smart"
	Severity  DiagnosticSeverity `json:"severity"`
	Message   string             `json:"message"`
}

// CheckConfig re-validates every enabled collector's prerequisites against
// what's actually on disk/in PATH right now. It deliberately only flags
// evidence of real misconfiguration (a binary that should exist and doesn't,
// a path that's set but unreadable, globs that match nothing) — never "this
// optional field was left empty", since that's a legitimate opt-out for most
// of these features and flagging it would just be noise. Called once per
// report cycle (cheap: stat/exec.LookPath calls only, no network).
func CheckConfig(cfg *config.Config) []DiagnosticIssue {
	// Always non-nil (even when empty) — the caller sends this on every
	// report without omitempty specifically so a fixed issue clears the
	// server-side cache instead of leaving a stale warning behind.
	issues := []DiagnosticIssue{}

	if cfg.CollectAPT {
		if _, err := exec.LookPath("apt-get"); err != nil {
			issues = append(issues, DiagnosticIssue{
				Collector: "apt", Severity: DiagnosticWarning,
				Message: "apt-get introuvable dans le PATH — collect_apt est activé mais ne remontera aucune donnée (normal sur un système non basé sur Debian/Ubuntu)",
			})
		}
	}

	if cfg.CollectDocker {
		if os.Getenv("DOCKER_HOST") == "" {
			if _, err := os.Stat("/var/run/docker.sock"); err != nil {
				issues = append(issues, DiagnosticIssue{
					Collector: "docker", Severity: DiagnosticWarning,
					Message: "socket Docker introuvable (/var/run/docker.sock) et DOCKER_HOST non défini — collect_docker est activé mais le client échouera à se connecter",
				})
			}
		}
	}

	if cfg.CollectSMART {
		if ok, detail := CheckSMARTAvailability(); !ok {
			issues = append(issues, DiagnosticIssue{Collector: "smart", Severity: DiagnosticWarning, Message: detail})
		}
	}

	if cfg.CollectCPUTemperature {
		if getCPUTemperature() <= 0 {
			issues = append(issues, DiagnosticIssue{
				Collector: "cpu_temperature", Severity: DiagnosticWarning,
				Message: "aucun capteur de température exploitable (ni /sys/class/thermal, ni /sys/class/hwmon, ni la commande sensors) — collect_cpu_temperature est activé mais ne remontera aucune donnée",
			})
		}
	}

	if cfg.CollectWebLogs {
		issues = append(issues, checkWebLogsConfig(cfg)...)
	}

	if cfg.CollectCrowdSecCorrelation && !cfg.CollectWebLogs {
		issues = append(issues, DiagnosticIssue{
			Collector: "crowdsec", Severity: DiagnosticWarning,
			Message: "collect_crowdsec_correlation est activé mais collect_web_logs ne l'est pas — la corrélation CrowdSec nécessite les logs web pour avoir des IP à vérifier",
		})
	}

	if cfg.CollectRestic {
		issues = append(issues, checkResticConfig(cfg)...)
	}

	if cfg.CollectNetworkFlows {
		if ok, reason := checkConntrackAcct(); !ok {
			issues = append(issues, DiagnosticIssue{
				Collector: "network_flows", Severity: DiagnosticWarning,
				Message: "collect_network_flows est activé mais aucun compteur d'octets par connexion n'est disponible : " + reason,
			})
		}
	}

	return issues
}

func checkWebLogsConfig(cfg *config.Config) []DiagnosticIssue {
	var issues []DiagnosticIssue
	if len(expandGlobs(cfg.WebLogGlobs())) == 0 {
		issues = append(issues, DiagnosticIssue{
			Collector: "web_logs", Severity: DiagnosticWarning,
			Message: "aucun fichier de log ne correspond aux motifs configurés (web_logs_log_paths) — collect_web_logs est activé mais ne trouvera rien à analyser",
		})
	}
	if cfg.WebLogsCursorFile != "" {
		if _, err := os.Stat(filepath.Dir(cfg.WebLogsCursorFile)); err != nil {
			issues = append(issues, DiagnosticIssue{
				Collector: "web_logs", Severity: DiagnosticWarning,
				Message: fmt.Sprintf("le dossier du fichier curseur n'existe pas : %s", filepath.Dir(cfg.WebLogsCursorFile)),
			})
		}
	}
	return issues
}

// checkResticConfig covers exactly the failure mode reported in the field:
// collect_restic: true with the rest of the restic_* keys left at their
// empty/default value — silently produces no passive status and no working
// manual/scheduled backup, with nothing surfaced until the first attempt.
func checkResticConfig(cfg *config.Config) []DiagnosticIssue {
	var issues []DiagnosticIssue

	bin := cfg.ResticBin
	if bin == "" {
		bin = "restic"
	}
	if _, err := exec.LookPath(bin); err != nil {
		if _, statErr := os.Stat(bin); statErr != nil {
			issues = append(issues, DiagnosticIssue{
				Collector: "restic", Severity: DiagnosticError,
				Message: fmt.Sprintf("binaire restic introuvable (%s) — ni dans le PATH, ni à ce chemin absolu", bin),
			})
		}
	}

	if cfg.ResticConfPath == "" {
		issues = append(issues, DiagnosticIssue{
			Collector: "restic", Severity: DiagnosticError,
			Message: "restic_conf_path n'est pas configuré — les identifiants du dépôt/backend ne seront jamais chargés, aucun statut ni backup ne fonctionnera",
		})
	} else if _, err := os.Stat(cfg.ResticConfPath); err != nil {
		issues = append(issues, DiagnosticIssue{
			Collector: "restic", Severity: DiagnosticError,
			Message: fmt.Sprintf("resticconf introuvable : %s", cfg.ResticConfPath),
		})
	}

	if cfg.ResticRunScriptPath == "" {
		issues = append(issues, DiagnosticIssue{
			Collector: "restic", Severity: DiagnosticWarning,
			Message: "restic_run_script_path n'est pas configuré — le déclenchement manuel/planifié de backup sera indisponible (le monitoring passif fonctionne quand même)",
		})
	} else if info, err := os.Stat(cfg.ResticRunScriptPath); err != nil {
		issues = append(issues, DiagnosticIssue{
			Collector: "restic", Severity: DiagnosticWarning,
			Message: fmt.Sprintf("run_backup.sh introuvable : %s", cfg.ResticRunScriptPath),
		})
	} else if info.Mode()&0o111 == 0 {
		issues = append(issues, DiagnosticIssue{
			Collector: "restic", Severity: DiagnosticWarning,
			Message: fmt.Sprintf("run_backup.sh n'est pas exécutable (chmod +x manquant) : %s", cfg.ResticRunScriptPath),
		})
	}

	if cfg.ResticProfileConfigPath != "" {
		if _, err := os.Stat(cfg.ResticProfileConfigPath); err != nil {
			issues = append(issues, DiagnosticIssue{
				Collector: "restic", Severity: DiagnosticWarning,
				Message: fmt.Sprintf("resticprofile.yaml introuvable : %s — les sélecteurs de profil/groupe resteront vides", cfg.ResticProfileConfigPath),
			})
		}
	}

	return issues
}
