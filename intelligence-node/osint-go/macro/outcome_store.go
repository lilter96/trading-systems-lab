package macro

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type OutcomeStore struct {
	db *sql.DB
}

func NewOutcomeStore(connStr string) (*OutcomeStore, error) {
	if connStr == "" {
		return nil, nil
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("outcome store: open db: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("outcome store: ping db: %w", err)
	}

	return &OutcomeStore{db: db}, nil
}

func (s *OutcomeStore) RecordOutcome(ctx context.Context, outcome EventOutcome) error {
	if s == nil || s.db == nil {
		return nil
	}

	query := `
		INSERT INTO macro_event_outcomes
			(event_id, event_name, instrument, source, surprise_magnitude,
			 realized_vol_pre, realized_vol_post, direction_accuracy, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	var dirAcc *bool
	if outcome.DirectionAccuracy != nil {
		dirAcc = outcome.DirectionAccuracy
	}

	_, err := s.db.ExecContext(ctx, query,
		outcome.EventID,
		outcome.EventName,
		outcome.Instrument,
		outcome.Source,
		outcome.SurpriseMagnitude,
		outcome.RealizedVolPre,
		outcome.RealizedVolPost,
		dirAcc,
		outcome.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("outcome store: insert: %w", err)
	}

	return nil
}

func (s *OutcomeStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
