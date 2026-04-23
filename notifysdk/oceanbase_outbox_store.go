package notifysdk

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OceanBaseOutboxStore struct {
	DB        *sql.DB
	TableName string
	Now       func() time.Time
}

func NewOceanBaseOutboxStore(db *sql.DB) *OceanBaseOutboxStore {
	return &OceanBaseOutboxStore{
		DB:        db,
		TableName: "notification_outbox",
		Now:       time.Now,
	}
}

func (s *OceanBaseOutboxStore) Save(ctx context.Context, msg OutboxMessage) error {
	return s.insert(ctx, s.DB, msg)
}

func (s *OceanBaseOutboxStore) SaveInTx(ctx context.Context, tx Tx, msg OutboxMessage) error {
	return s.insert(ctx, tx, msg)
}

func (s *OceanBaseOutboxStore) ClaimPending(ctx context.Context, limit int, now time.Time) ([]OutboxMessage, error) {
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`
select
  message_id,
  biz_type,
  event_code,
  template_code,
  receiver_json,
  payload_json,
  idempotent_key,
  trace_id,
  priority,
  headers_json,
  meta_json,
  status,
  retry_count,
  next_retry_at,
  last_error,
  created_at,
  updated_at
from %s
where status = ?
  and next_retry_at <= ?
order by next_retry_at asc, created_at asc, message_id asc
limit ?`, s.tableName()), OutboxPending, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]OutboxMessage, 0, limit)
	for rows.Next() {
		msg, err := scanOutboxMessage(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	claimed := make([]OutboxMessage, 0, len(candidates))
	for _, item := range candidates {
		ok, err := s.tryMarkSending(ctx, item.ID, now)
		if err != nil {
			return nil, err
		}
		if ok {
			item.Status = OutboxSending
			item.UpdatedAt = now
			claimed = append(claimed, item)
		}
	}

	return claimed, nil
}

func (s *OceanBaseOutboxStore) MarkSent(ctx context.Context, id string, now time.Time) error {
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
update %s
set status = ?, last_error = '', updated_at = ?
where message_id = ?`, s.tableName()), OutboxSent, now, id)
	return err
}

func (s *OceanBaseOutboxStore) MarkRetry(ctx context.Context, id string, retryCount int, next time.Time, lastError string) error {
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
update %s
set status = ?, retry_count = ?, next_retry_at = ?, last_error = ?, updated_at = ?
where message_id = ?`, s.tableName()), OutboxPending, retryCount, next, truncate(lastError, 1024), next, id)
	return err
}

func (s *OceanBaseOutboxStore) MarkDead(ctx context.Context, id string, retryCount int, now time.Time, lastError string) error {
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
update %s
set status = ?, retry_count = ?, last_error = ?, updated_at = ?
where message_id = ?`, s.tableName()), OutboxDead, retryCount, truncate(lastError, 1024), now, id)
	return err
}

func (s *OceanBaseOutboxStore) CountByStatus(ctx context.Context, status OutboxStatus) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, fmt.Sprintf(`
select count(1)
from %s
where status = ?`, s.tableName()), status).Scan(&count)
	return count, err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type outboxScanner interface {
	Scan(dest ...any) error
}

func (s *OceanBaseOutboxStore) insert(ctx context.Context, exec execer, msg OutboxMessage) error {
	receiverJSON, err := json.Marshal(msg.Envelope.Receivers)
	if err != nil {
		return wrapError(ErrMarshalPayload, err)
	}

	headersJSON, err := marshalOptionalMap(msg.Envelope.Headers)
	if err != nil {
		return wrapError(ErrMarshalPayload, err)
	}

	metaJSON, err := marshalOptionalMap(msg.Envelope.Meta)
	if err != nil {
		return wrapError(ErrMarshalPayload, err)
	}

	_, err = exec.ExecContext(ctx, fmt.Sprintf(`
insert into %s (
  message_id,
  biz_type,
  event_code,
  template_code,
  receiver_json,
  payload_json,
  idempotent_key,
  trace_id,
  priority,
  headers_json,
  meta_json,
  status,
  retry_count,
  next_retry_at,
  last_error,
  created_at,
  updated_at
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.tableName()),
		msg.ID,
		msg.Envelope.BizType,
		msg.Envelope.EventCode,
		msg.Envelope.TemplateCode,
		string(receiverJSON),
		string(msg.Envelope.Payload),
		msg.Envelope.IdempotentKey,
		nullIfEmpty(msg.Envelope.TraceID),
		msg.Envelope.Priority,
		headersJSON,
		metaJSON,
		msg.Status,
		msg.RetryCount,
		msg.NextAttemptAt,
		nullIfEmpty(truncate(msg.LastError, 1024)),
		msg.CreatedAt,
		msg.UpdatedAt,
	)
	return err
}

func (s *OceanBaseOutboxStore) tryMarkSending(ctx context.Context, id string, now time.Time) (bool, error) {
	result, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
update %s
set status = ?, updated_at = ?
where message_id = ?
  and status = ?`, s.tableName()), OutboxSending, now, id, OutboxPending)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (s *OceanBaseOutboxStore) tableName() string {
	name := strings.TrimSpace(s.TableName)
	if name == "" {
		return "notification_outbox"
	}
	return name
}

func scanOutboxMessage(scanner outboxScanner) (OutboxMessage, error) {
	var (
		messageID    string
		bizType      string
		eventCode    string
		templateCode string
		receiverJSON string
		payloadJSON  []byte
		idempotent   string
		traceID      sql.NullString
		priority     int
		headersJSON  sql.NullString
		metaJSON     sql.NullString
		status       string
		retryCount   int
		nextRetryAt  time.Time
		lastError    sql.NullString
		createdAt    time.Time
		updatedAt    time.Time
	)

	if err := scanner.Scan(
		&messageID,
		&bizType,
		&eventCode,
		&templateCode,
		&receiverJSON,
		&payloadJSON,
		&idempotent,
		&traceID,
		&priority,
		&headersJSON,
		&metaJSON,
		&status,
		&retryCount,
		&nextRetryAt,
		&lastError,
		&createdAt,
		&updatedAt,
	); err != nil {
		return OutboxMessage{}, err
	}

	receivers := make([]Receiver, 0)
	if err := json.Unmarshal([]byte(receiverJSON), &receivers); err != nil {
		return OutboxMessage{}, err
	}

	headers, err := unmarshalOptionalMap(headersJSON)
	if err != nil {
		return OutboxMessage{}, err
	}

	meta, err := unmarshalOptionalMap(metaJSON)
	if err != nil {
		return OutboxMessage{}, err
	}

	return OutboxMessage{
		ID: messageID,
		Envelope: Envelope{
			BizType:       bizType,
			EventCode:     eventCode,
			TemplateCode:  templateCode,
			Receivers:     receivers,
			Payload:       append([]byte(nil), payloadJSON...),
			IdempotentKey: idempotent,
			TraceID:       traceID.String,
			Priority:      priority,
			Headers:       headers,
			Meta:          meta,
		},
		Status:        OutboxStatus(status),
		RetryCount:    retryCount,
		NextAttemptAt: nextRetryAt,
		LastError:     lastError.String,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}

func marshalOptionalMap(data map[string]string) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func unmarshalOptionalMap(raw sql.NullString) (map[string]string, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(raw.String), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
