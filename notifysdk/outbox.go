package notifysdk

import (
	"context"
	"fmt"
	"time"
)

type OutboxStatus string

const (
	OutboxPending OutboxStatus = "pending"
	OutboxSending OutboxStatus = "sending"
	OutboxSent    OutboxStatus = "sent"
	OutboxDead    OutboxStatus = "dead"
)

type OutboxMessage struct {
	ID            string
	Envelope      Envelope
	Status        OutboxStatus
	RetryCount    int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OutboxStore interface {
	Save(context.Context, OutboxMessage) error
	ClaimPending(context.Context, int, time.Time) ([]OutboxMessage, error)
	MarkSent(context.Context, string, time.Time) error
	MarkRetry(context.Context, string, int, time.Time, string) error
	MarkDead(context.Context, string, int, time.Time, string) error
}

type OutboxTxStore interface {
	SaveInTx(context.Context, Tx, OutboxMessage) error
}

type OutboxStatsReader interface {
	CountByStatus(context.Context, OutboxStatus) (int, error)
}

type OutboxTransport struct {
	Store OutboxStore
	Now   func() time.Time
}

func (t *OutboxTransport) Name() Mode {
	return ModeOutbox
}

func (t *OutboxTransport) Send(ctx context.Context, envelope Envelope) (Result, error) {
	now := time.Now
	if t.Now != nil {
		now = t.Now
	}

	message := OutboxMessage{
		ID:            envelope.IdempotentKey,
		Envelope:      envelope,
		Status:        OutboxPending,
		RetryCount:    0,
		NextAttemptAt: now(),
		CreatedAt:     now(),
		UpdatedAt:     now(),
	}

	if err := t.Store.Save(ctx, message); err != nil {
		return Result{}, wrapError(ErrEnqueueFailed, err)
	}

	return Result{
		Accepted:     true,
		RequestID:    message.ID,
		DeliveryMode: ModeOutbox,
		Queued:       true,
	}, nil
}

type RetryPolicy interface {
	NextRetry(retryCount int, now time.Time) (time.Time, bool)
}

type FixedDelays struct {
	Delays []time.Duration
}

func (p FixedDelays) NextRetry(retryCount int, now time.Time) (time.Time, bool) {
	if retryCount <= 0 || retryCount > len(p.Delays) {
		return time.Time{}, false
	}

	return now.Add(p.Delays[retryCount-1]), true
}

type DispatchReport struct {
	Processed int
	Sent      int
	Retried   int
	Dead      int
}

type Dispatcher struct {
	Store       OutboxStore
	Relay       Transport
	RetryPolicy RetryPolicy
	Now         func() time.Time
	Metrics     Metrics
}

func (d *Dispatcher) DispatchOnce(ctx context.Context, limit int) (DispatchReport, error) {
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}

	items, err := d.Store.ClaimPending(ctx, limit, now())
	if err != nil {
		d.metrics().RecordError(ModeOutbox, "claim_pending_failed")
		return DispatchReport{}, err
	}

	report := DispatchReport{}
	for _, item := range items {
		report.Processed++

		if _, err := d.Relay.Send(ctx, item.Envelope); err != nil {
			d.metrics().RecordError(d.Relay.Name(), classifyError(err))
			reportRetry, retryErr := d.handleFailure(ctx, item, now(), err)
			report.Retried += reportRetry.Retried
			report.Dead += reportRetry.Dead
			if retryErr != nil {
				d.metrics().RecordError(ModeOutbox, "store_update_failed")
				return report, retryErr
			}
			continue
		}

		if err := d.Store.MarkSent(ctx, item.ID, now()); err != nil {
			d.metrics().RecordError(ModeOutbox, "store_update_failed")
			return report, err
		}
		report.Sent++
		d.metrics().RecordDispatch("sent")
	}

	d.refreshOutboxMetrics(ctx)

	return report, nil
}

func (d *Dispatcher) handleFailure(ctx context.Context, item OutboxMessage, now time.Time, cause error) (DispatchReport, error) {
	if d.RetryPolicy == nil {
		if err := d.Store.MarkDead(ctx, item.ID, item.RetryCount+1, now, cause.Error()); err != nil {
			return DispatchReport{}, err
		}
		d.metrics().RecordDispatch("dead")
		return DispatchReport{Dead: 1}, nil
	}

	next, ok := d.RetryPolicy.NextRetry(item.RetryCount+1, now)
	if !ok {
		if err := d.Store.MarkDead(ctx, item.ID, item.RetryCount+1, now, cause.Error()); err != nil {
			return DispatchReport{}, err
		}
		d.metrics().RecordDispatch("dead")
		return DispatchReport{Dead: 1}, nil
	}

	if err := d.Store.MarkRetry(ctx, item.ID, item.RetryCount+1, next, cause.Error()); err != nil {
		return DispatchReport{}, err
	}

	d.metrics().RecordDispatch("retry")
	return DispatchReport{Retried: 1}, nil
}

func (d *Dispatcher) metrics() Metrics {
	if d.Metrics == nil {
		return NoopMetrics{}
	}
	return d.Metrics
}

func (d *Dispatcher) refreshOutboxMetrics(ctx context.Context) {
	stats, ok := d.Store.(OutboxStatsReader)
	if !ok {
		return
	}

	pending, err := stats.CountByStatus(ctx, OutboxPending)
	if err == nil {
		d.metrics().SetOutboxSize(OutboxPending, pending)
	}

	dead, err := stats.CountByStatus(ctx, OutboxDead)
	if err == nil {
		d.metrics().SetOutboxSize(OutboxDead, dead)
	}
}

type MemoryOutboxStore struct {
	items map[string]OutboxMessage
}

func NewMemoryOutboxStore() *MemoryOutboxStore {
	return &MemoryOutboxStore{
		items: make(map[string]OutboxMessage),
	}
}

func (s *MemoryOutboxStore) Save(_ context.Context, msg OutboxMessage) error {
	if _, exists := s.items[msg.ID]; exists {
		return fmt.Errorf("%w: duplicate idempotent key %s", ErrEnqueueFailed, msg.ID)
	}
	s.items[msg.ID] = msg
	return nil
}

func (s *MemoryOutboxStore) SaveInTx(ctx context.Context, _ Tx, msg OutboxMessage) error {
	return s.Save(ctx, msg)
}

func (s *MemoryOutboxStore) ClaimPending(_ context.Context, limit int, now time.Time) ([]OutboxMessage, error) {
	result := make([]OutboxMessage, 0, limit)
	for id, item := range s.items {
		if len(result) >= limit {
			break
		}
		if item.Status != OutboxPending || item.NextAttemptAt.After(now) {
			continue
		}
		item.Status = OutboxSending
		item.UpdatedAt = now
		s.items[id] = item
		result = append(result, item)
	}
	return result, nil
}

func (s *MemoryOutboxStore) MarkSent(_ context.Context, id string, now time.Time) error {
	item, ok := s.items[id]
	if !ok {
		return ErrOutboxNotFound
	}
	item.Status = OutboxSent
	item.LastError = ""
	item.UpdatedAt = now
	s.items[id] = item
	return nil
}

func (s *MemoryOutboxStore) MarkRetry(_ context.Context, id string, retryCount int, next time.Time, lastError string) error {
	item, ok := s.items[id]
	if !ok {
		return ErrOutboxNotFound
	}
	item.Status = OutboxPending
	item.RetryCount = retryCount
	item.NextAttemptAt = next
	item.LastError = lastError
	item.UpdatedAt = next
	s.items[id] = item
	return nil
}

func (s *MemoryOutboxStore) MarkDead(_ context.Context, id string, retryCount int, now time.Time, lastError string) error {
	item, ok := s.items[id]
	if !ok {
		return ErrOutboxNotFound
	}
	item.Status = OutboxDead
	item.RetryCount = retryCount
	item.LastError = lastError
	item.UpdatedAt = now
	s.items[id] = item
	return nil
}

func (s *MemoryOutboxStore) CountByStatus(_ context.Context, status OutboxStatus) (int, error) {
	count := 0
	for _, item := range s.items {
		if item.Status == status {
			count++
		}
	}
	return count, nil
}
