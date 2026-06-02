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

type Dispatcher struct {
	db *database.DB
}

func New(db *database.DB) *Dispatcher {
	return &Dispatcher{db: db}
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

	return &Result{Command: cmd, AuditLogID: auditLogIDPtr}, nil
}
