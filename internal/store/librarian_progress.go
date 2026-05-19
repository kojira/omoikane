package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// LibrarianProgress is one row of librarian_progress.
type LibrarianProgress struct {
	ID            int64     `json:"id"`
	Role          string    `json:"role"`
	EntryID       string    `json:"entry_id"`
	InstanceID    string    `json:"instance_id,omitempty"`
	ProcessedAt   time.Time `json:"processed_at"`
	Action        string    `json:"action"`
	OutputEntryID string    `json:"output_entry_id,omitempty"`
	Notes         string    `json:"notes,omitempty"`
}

// NextUnprocessedEntry returns the oldest entry that has no
// librarian_progress row for the given role. Returns ErrNotFound when
// the role has caught up to current (no backlog left).
//
// The status filter accepts ACTIVE and DRAFT by default; SUPERSEDED /
// ARCHIVED / DUPLICATE entries are excluded because re-processing
// them would just churn — they're already settled. If a caller needs
// to backfill processing of historically-archived entries they can
// extend this query later.
//
// Projects optionally filters by project_id (empty = all projects).
func (s *Store) NextUnprocessedEntry(ctx context.Context, role, projectID string) (*Entry, error) {
	if !ValidLibrarianRole(role) {
		return nil, fmt.Errorf("%w: role %q", ErrInvalidInput, role)
	}
	// entrySelectSQL already includes "FROM entries e"; we add the
	// LEFT JOIN + WHERE inline and let scanEntry consume the row.
	args := []any{role}
	q := entrySelectSQL + `
		LEFT JOIN librarian_progress lp
		  ON lp.entry_id = e.id AND lp.role = ?
		WHERE lp.id IS NULL
		  AND e.status IN ('ACTIVE','DRAFT')`
	if projectID != "" {
		q += ` AND e.project_id = ?`
		args = append(args, projectID)
	}
	q += ` ORDER BY e.created_at ASC LIMIT 1`

	row := s.db.QueryRowContext(ctx, q, args...)
	e, err := scanEntry(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

// RecordProgress writes a librarian_progress row marking that the
// given role has processed (or explicitly chose not to act on) the
// given entry.
//
// Validation:
//   - role must be a canonical librarian role
//   - entry_id must exist
//   - action must be non-blank (vocabulary is role-specific and
//     validated at the API layer, not here, so new actions can ship
//     without a store-level whitelist change)
func (s *Store) RecordProgress(ctx context.Context, p *LibrarianProgress) error {
	if !ValidLibrarianRole(p.Role) {
		return fmt.Errorf("%w: role %q", ErrInvalidInput, p.Role)
	}
	if strings.TrimSpace(p.EntryID) == "" {
		return fmt.Errorf("%w: entry_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(p.Action) == "" {
		return fmt.Errorf("%w: action required", ErrInvalidInput)
	}
	// Verify entry exists (don't accumulate progress rows for phantom IDs).
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM entries WHERE id = ?`, p.EntryID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: entry %s", ErrNotFound, p.EntryID)
	}
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO librarian_progress(role, entry_id, instance_id, action, output_entry_id, notes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.Role, p.EntryID, nullable(p.InstanceID), p.Action,
		nullable(p.OutputEntryID), nullable(p.Notes))
	if err != nil {
		return translateErr(err)
	}
	id, _ := res.LastInsertId()
	p.ID = id
	// Read back processed_at for parity with other store inserts.
	_ = s.db.QueryRowContext(ctx,
		`SELECT processed_at FROM librarian_progress WHERE id = ?`, id).Scan(&p.ProcessedAt)
	return nil
}

// ListProgress returns the most recent N progress rows for a role,
// optionally filtered by instance.
func (s *Store) ListProgress(ctx context.Context, role, instanceID string, limit int) ([]*LibrarianProgress, error) {
	if !ValidLibrarianRole(role) {
		return nil, fmt.Errorf("%w: role %q", ErrInvalidInput, role)
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	args := []any{role}
	q := `
		SELECT id, role, entry_id, COALESCE(instance_id,''),
		       processed_at, action, COALESCE(output_entry_id,''),
		       COALESCE(notes,'')
		FROM librarian_progress
		WHERE role = ?`
	if instanceID != "" {
		q += ` AND instance_id = ?`
		args = append(args, instanceID)
	}
	// Tie-break by id DESC since SQLite's CURRENT_TIMESTAMP is
	// second-resolution and multiple inserts within one tick share
	// a processed_at value. id is autoincrement so newer rows always
	// have higher ids.
	q += ` ORDER BY processed_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*LibrarianProgress{}
	for rows.Next() {
		var p LibrarianProgress
		if err := rows.Scan(&p.ID, &p.Role, &p.EntryID, &p.InstanceID,
			&p.ProcessedAt, &p.Action, &p.OutputEntryID, &p.Notes); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// BacklogSize returns the count of entries with no librarian_progress
// row for the given role — i.e. how much work this role has
// outstanding. Cheap query; used by coordinator's triage and by
// dashboards.
func (s *Store) BacklogSize(ctx context.Context, role, projectID string) (int, error) {
	if !ValidLibrarianRole(role) {
		return 0, fmt.Errorf("%w: role %q", ErrInvalidInput, role)
	}
	args := []any{role}
	q := `
		SELECT COUNT(*)
		FROM entries e
		LEFT JOIN librarian_progress lp
		  ON lp.entry_id = e.id AND lp.role = ?
		WHERE lp.id IS NULL
		  AND e.status IN ('ACTIVE','DRAFT')`
	if projectID != "" {
		q += ` AND e.project_id = ?`
		args = append(args, projectID)
	}
	var n int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}
