package mssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

type AuditStore struct {
	db *DB
}

func NewAuditStore(db *DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Create(ctx context.Context, log *model.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, request_id, action, actor_id, details, created_at)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6)`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now().UTC()

	_, err := s.db.Pool.ExecContext(ctx, query,
		log.ID, log.RequestID, log.Action, log.ActorID, nullableString(log.Details), log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	return nil
}

func (s *AuditStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.AuditLog, error) {
	query := `
		SELECT id, request_id, action, actor_id, details, created_at
		FROM audit_logs WHERE request_id = @p1 ORDER BY created_at ASC`

	rows, err := s.db.Pool.QueryContext(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("listing audit logs: %w", err)
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var l model.AuditLog
		var details sql.NullString
		if err := rows.Scan(&l.ID, &l.RequestID, &l.Action, &l.ActorID, &details, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit log: %w", err)
		}
		if details.Valid {
			l.Details = json.RawMessage(details.String)
		}
		logs = append(logs, l)
	}

	return logs, nil
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
