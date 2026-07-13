package dispatch

import (
	"context"
	"log/slog"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AuditLogRequest struct {
	Username  string
	Action    string
	HostID    string
	IPAddress string
	Details   string
}

type Request struct {
	HostID      string
	Module      string
	Action      string
	Target      string
	Payload     string
	TriggeredBy string
	Audit       *AuditLogRequest
}

type Result struct {
	Command    *models.RemoteCommand
	AuditLogID *int64
}

// AgentPusher lets the dispatcher nudge a live agent WebSocket connection to
// poll immediately after a command is created, instead of the agent waiting
// for its next scheduled poll. Optional (nil-safe) and best-effort: Create()
// always persists the command to remote_commands first regardless, so an
// agent with no live push connection is unaffected — it picks the command up
// on its next poll exactly as before. Deliberately just a nudge (no command
// content crosses this interface): the agent's existing poll/claim query is
// already atomic (FOR UPDATE SKIP LOCKED), so triggering it early is safe,
// where pushing command content over a second path would race it and risk
// double delivery. *ws.AgentHub satisfies this structurally; dispatch does
// not import ws to avoid a cycle (ws already needs types from several
// packages that would sit awkwardly importing dispatch back).
type AgentPusher interface {
	Notify(hostID string) bool
}

type Dispatcher struct {
	db     *database.DB
	pusher AgentPusher
}

func New(db *database.DB) *Dispatcher {
	return &Dispatcher{db: db}
}

// SetAgentPusher wires the optional live-push channel after construction —
// mirrors AgentHandler.AddCompletionListener's shape, since the pusher
// (ws.AgentHub) and the dispatcher are constructed in different places and
// only one of them needs to exist first.
func (d *Dispatcher) SetAgentPusher(pusher AgentPusher) {
	d.pusher = pusher
}

func (d *Dispatcher) Create(ctx context.Context, req Request) (*Result, error) {
	var auditLogIDPtr *int64
	if req.Audit != nil {
		auditLogID, err := d.db.CreateAuditLog(ctx,
			req.Audit.Username,
			req.Audit.Action,
			req.Audit.HostID,
			req.Audit.IPAddress,
			req.Audit.Details,
			"pending",
		)
		if err != nil {
			slog.WarnContext(ctx, "failed to create audit log for command", slog.String("module", req.Module), slog.String("action", req.Action), slog.Any("err", err))
		} else {
			auditLogIDPtr = &auditLogID
		}
	}

	cmd, err := d.db.CreateRemoteCommand(ctx,
		req.HostID,
		req.Module,
		req.Action,
		req.Target,
		req.Payload,
		req.TriggeredBy,
		auditLogIDPtr,
	)
	if err != nil {
		if auditLogIDPtr != nil {
			_ = d.db.UpdateAuditLogStatus(ctx, *auditLogIDPtr, "failed", err.Error())
		}
		return nil, err
	}

	if d.pusher != nil {
		d.pusher.Notify(req.HostID)
	}

	return &Result{Command: cmd, AuditLogID: auditLogIDPtr}, nil
}
