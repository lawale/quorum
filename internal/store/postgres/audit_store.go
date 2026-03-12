package postgres

import (
	"context"
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
		VALUES ($1, $2, $3, $4, $5, $6)`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now().UTC()

	_, err := s.db.Pool.Exec(ctx, query,
		log.ID, log.RequestID, log.Action, log.ActorID, log.Details, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	return nil
}

func (s *AuditStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.AuditLog, error) {
	query := `
		SELECT id, request_id, action, actor_id, details, created_at
		FROM audit_logs WHERE request_id = $1 ORDER BY created_at ASC`

	rows, err := s.db.Pool.Query(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("listing audit logs: %w", err)
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var l model.AuditLog
		if err := rows.Scan(&l.ID, &l.RequestID, &l.Action, &l.ActorID, &l.Details, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit log: %w", err)
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
