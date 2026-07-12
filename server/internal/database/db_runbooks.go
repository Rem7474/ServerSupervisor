package database

import (
	"context"
	"database/sql"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Runbooks (definitions) ==========

// CreateRunbook inserts a runbook and its ordered steps in one transaction.
func (db *DB) CreateRunbook(ctx context.Context, name, description string, steps []models.RunbookStepCreate) (*models.Runbook, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rb := models.Runbook{Name: name, Description: description}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO runbooks (name, description) VALUES ($1, $2) RETURNING id, created_at, updated_at`,
		name, description,
	).Scan(&rb.ID, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
		return nil, err
	}

	if err := insertRunbookSteps(ctx, tx, rb.ID, steps); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	rb.Steps = stepCreatesToSteps(rb.ID, steps)
	return &rb, nil
}

// insertRunbookSteps inserts steps in order starting at position 0.
func insertRunbookSteps(ctx context.Context, tx *sql.Tx, runbookID string, steps []models.RunbookStepCreate) error {
	for i, step := range steps {
		payload := step.Payload
		if payload == "" {
			payload = "{}"
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO runbook_steps (runbook_id, position, host_id, module, action, target, payload, continue_on_failure)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			runbookID, i, step.HostID, step.Module, step.Action, step.Target, payload, step.ContinueOnFailure,
		); err != nil {
			return err
		}
	}
	return nil
}

func stepCreatesToSteps(runbookID string, steps []models.RunbookStepCreate) []models.RunbookStep {
	out := make([]models.RunbookStep, 0, len(steps))
	for i, s := range steps {
		payload := s.Payload
		if payload == "" {
			payload = "{}"
		}
		out = append(out, models.RunbookStep{
			RunbookID: runbookID, Position: i, HostID: s.HostID, Module: s.Module,
			Action: s.Action, Target: s.Target, Payload: payload, ContinueOnFailure: s.ContinueOnFailure,
		})
	}
	return out
}

// GetRunbook returns a runbook with its ordered steps, or sql.ErrNoRows.
func (db *DB) GetRunbook(ctx context.Context, id string) (*models.Runbook, error) {
	var rb models.Runbook
	if err := db.conn.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM runbooks WHERE id = $1`, id,
	).Scan(&rb.ID, &rb.Name, &rb.Description, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
		return nil, err
	}
	steps, err := db.listRunbookSteps(ctx, id)
	if err != nil {
		return nil, err
	}
	rb.Steps = steps
	return &rb, nil
}

// ListRunbooks returns all runbooks (newest first) with their ordered steps, never nil.
func (db *DB) ListRunbooks(ctx context.Context) ([]models.Runbook, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM runbooks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	runbooks := []models.Runbook{}
	for rows.Next() {
		var rb models.Runbook
		if err := rows.Scan(&rb.ID, &rb.Name, &rb.Description, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
			return nil, err
		}
		runbooks = append(runbooks, rb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range runbooks {
		steps, err := db.listRunbookSteps(ctx, runbooks[i].ID)
		if err != nil {
			return nil, err
		}
		runbooks[i].Steps = steps
	}
	return runbooks, nil
}

func (db *DB) listRunbookSteps(ctx context.Context, runbookID string) ([]models.RunbookStep, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, runbook_id, position, host_id, module, action, target, payload, continue_on_failure
		 FROM runbook_steps WHERE runbook_id = $1 ORDER BY position ASC`, runbookID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	steps := []models.RunbookStep{}
	for rows.Next() {
		var s models.RunbookStep
		if err := rows.Scan(&s.ID, &s.RunbookID, &s.Position, &s.HostID, &s.Module, &s.Action, &s.Target, &s.Payload, &s.ContinueOnFailure); err != nil {
			return nil, err
		}
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

// GetRunbookStepByPosition returns sql.ErrNoRows when the runbook has no step at that position.
func (db *DB) GetRunbookStepByPosition(ctx context.Context, runbookID string, position int) (*models.RunbookStep, error) {
	var s models.RunbookStep
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, runbook_id, position, host_id, module, action, target, payload, continue_on_failure
		 FROM runbook_steps WHERE runbook_id = $1 AND position = $2`, runbookID, position,
	).Scan(&s.ID, &s.RunbookID, &s.Position, &s.HostID, &s.Module, &s.Action, &s.Target, &s.Payload, &s.ContinueOnFailure)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateRunbook applies partial field changes and, when steps is non-nil,
// replaces the step list wholesale (simplest correct semantics — a runbook's
// step list is small and edited as a unit in the UI, not diffed field by field).
func (db *DB) UpdateRunbook(ctx context.Context, id string, name, description *string, steps *[]models.RunbookStepCreate) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if name != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE runbooks SET name = $1, updated_at = NOW() WHERE id = $2`, *name, id); err != nil {
			return err
		}
	}
	if description != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE runbooks SET description = $1, updated_at = NOW() WHERE id = $2`, *description, id); err != nil {
			return err
		}
	}
	if steps != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM runbook_steps WHERE runbook_id = $1`, id); err != nil {
			return err
		}
		if err := insertRunbookSteps(ctx, tx, id, *steps); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE runbooks SET updated_at = NOW() WHERE id = $1`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteRunbook removes a runbook and its steps (ON DELETE CASCADE); past
// executions and their remote_commands rows are kept for history, with
// runbook_id / runbook_execution_id left dangling-safe by the FK's own
// ON DELETE CASCADE on runbook_executions — see the migration.
func (db *DB) DeleteRunbook(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM runbooks WHERE id = $1`, id)
	return err
}

// ========== Runbook executions ==========

func (db *DB) CreateRunbookExecution(ctx context.Context, runbookID, triggeredBy string) (*models.RunbookExecution, error) {
	var exec models.RunbookExecution
	exec.RunbookID = runbookID
	exec.TriggeredBy = triggeredBy
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO runbook_executions (runbook_id, triggered_by) VALUES ($1, $2)
		 RETURNING id, status, current_step_position, started_at`,
		runbookID, triggeredBy,
	).Scan(&exec.ID, &exec.Status, &exec.CurrentStepPosition, &exec.StartedAt)
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

func (db *DB) GetRunbookExecution(ctx context.Context, id string) (*models.RunbookExecution, error) {
	var exec models.RunbookExecution
	var completedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, runbook_id, status, current_step_position, triggered_by, started_at, completed_at
		 FROM runbook_executions WHERE id = $1`, id,
	).Scan(&exec.ID, &exec.RunbookID, &exec.Status, &exec.CurrentStepPosition, &exec.TriggeredBy, &exec.StartedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		exec.CompletedAt = &completedAt.Time
	}
	return &exec, nil
}

// ListRunbookExecutions returns a runbook's execution history, newest first, never nil.
func (db *DB) ListRunbookExecutions(ctx context.Context, runbookID string, limit int) ([]models.RunbookExecution, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, runbook_id, status, current_step_position, triggered_by, started_at, completed_at
		 FROM runbook_executions WHERE runbook_id = $1 ORDER BY started_at DESC LIMIT $2`, runbookID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	execs := []models.RunbookExecution{}
	for rows.Next() {
		var exec models.RunbookExecution
		var completedAt sql.NullTime
		if err := rows.Scan(&exec.ID, &exec.RunbookID, &exec.Status, &exec.CurrentStepPosition, &exec.TriggeredBy, &exec.StartedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			exec.CompletedAt = &completedAt.Time
		}
		execs = append(execs, exec)
	}
	return execs, rows.Err()
}

// AdvanceRunbookExecution moves an in-progress execution to the given step position.
func (db *DB) AdvanceRunbookExecution(ctx context.Context, executionID string, position int) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE runbook_executions SET current_step_position = $1 WHERE id = $2`, position, executionID)
	return err
}

// FinishRunbookExecution marks an execution terminal (status is "completed" or "failed").
func (db *DB) FinishRunbookExecution(ctx context.Context, executionID, status string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE runbook_executions SET status = $1, completed_at = NOW() WHERE id = $2`, status, executionID)
	return err
}

// LinkCommandToRunbookExecution associates a dispatched command with the
// runbook execution step that triggered it — mirrors LinkCommandToScheduledTask.
func (db *DB) LinkCommandToRunbookExecution(ctx context.Context, commandID, executionID string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE remote_commands SET runbook_execution_id = $1 WHERE id = $2`, executionID, commandID)
	return err
}

// ListRunbookExecutionSteps zips the runbook's step definitions with the
// commands this execution actually dispatched, ordered by creation time —
// steps run strictly sequentially (one waits for the previous to finish), so
// the Nth dispatched command always corresponds to the Nth step reached.
// Steps not yet reached come back with an empty Status/CommandID.
func (db *DB) ListRunbookExecutionSteps(ctx context.Context, executionID string) ([]models.RunbookExecutionStep, error) {
	exec, err := db.GetRunbookExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	steps, err := db.listRunbookSteps(ctx, exec.RunbookID)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, status, output FROM remote_commands WHERE runbook_execution_id = $1 ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type cmdOutcome struct {
		id, status, output string
	}
	var cmds []cmdOutcome
	for rows.Next() {
		var c cmdOutcome
		if err := rows.Scan(&c.id, &c.status, &c.output); err != nil {
			return nil, err
		}
		cmds = append(cmds, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.RunbookExecutionStep, 0, len(steps))
	for i, step := range steps {
		es := models.RunbookExecutionStep{
			Position: step.Position,
			HostID:   step.HostID,
			Module:   step.Module,
			Action:   step.Action,
			Target:   step.Target,
		}
		if i < len(cmds) {
			id := cmds[i].id
			es.CommandID = &id
			es.Status = cmds[i].status
			es.Output = cmds[i].output
		}
		out = append(out, es)
	}
	return out, nil
}
