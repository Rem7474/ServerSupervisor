package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

func handleCrowdSec(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	reportRunning(ctx, s, cmd)

	var output string
	var execErr error

	switch cmd.Action {
	case "unban":
		if cmd.Target == "" {
			execErr = fmt.Errorf("no IP provided for unban action")
		} else {
			execErr = collector.DeleteCrowdSecDecision(
				d.cfg.CrowdSecConnectionString,
				d.cfg.CrowdSecAlertsMachineID,
				d.cfg.CrowdSecAlertsPassword,
				cmd.Target,
			)
		}
	case "ban":
		if cmd.Target == "" {
			execErr = fmt.Errorf("no IP provided for ban action")
		} else {
			var banPayload struct {
				Duration string `json:"duration"`
			}
			banPayload.Duration = "4h"
			_ = json.Unmarshal([]byte(cmd.Payload), &banPayload)
			if banPayload.Duration == "" {
				banPayload.Duration = "4h"
			}
			execErr = collector.CreateCrowdSecDecision(
				d.cfg.CrowdSecConnectionString,
				d.cfg.CrowdSecAlertsMachineID,
				d.cfg.CrowdSecAlertsPassword,
				cmd.Target,
				banPayload.Duration,
			)
		}
	default:
		execErr = fmt.Errorf("unknown crowdsec action: %s", cmd.Action)
	}

	status := "completed"
	if execErr != nil {
		status = "failed"
		output = fmt.Sprintf("ERROR: %v", execErr)
		log.Printf("crowdsec %s %s failed: %v", cmd.Action, cmd.Target, execErr)
	} else {
		switch cmd.Action {
		case "ban":
			output = fmt.Sprintf("Successfully banned IP: %s", cmd.Target)
		default:
			output = fmt.Sprintf("Successfully unbanned IP: %s", cmd.Target)
		}
		log.Printf("crowdsec %s %s completed", cmd.Action, cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
