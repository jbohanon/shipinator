package postgres

import (
	"errors"
	"fmt"
	"time"

	"git.nonahob.net/jacob/shipinator/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func ensureID[T ~[16]byte](id *T) {
	if *id == (T{}) {
		*id = T([16]byte(uuid.New()))
	}
}

func toUUID[T ~[16]byte](id T) uuid.UUID {
	return uuid.UUID([16]byte(id))
}

func setCreatedAt(createdAt *time.Time) {
	*createdAt = time.Now()
}

func setUpdatedAt(updatedAt *time.Time) {
	*updatedAt = time.Now()
}

func setCreatedUpdated(createdAt, updatedAt *time.Time) {
	now := time.Now()
	*createdAt = now
	*updatedAt = now
}

func setDefaultStatus(status *string, fallback string) {
	if *status == "" {
		*status = fallback
	}
}

func wrapNoRowsByID[T ~[16]byte](entity string, id T, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s %s: %w", entity, uuid.UUID(id), store.ErrNotFound)
	}
	return err
}

func wrapNoRowsByName(entity, name string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s %q: %w", entity, name, store.ErrNotFound)
	}
	return err
}

func ensureRowsAffected[T ~[16]byte](result pgconn.CommandTag, entity string, id T) error {
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s %s: %w", entity, uuid.UUID(id), store.ErrNotFound)
	}
	return nil
}

func startedFinishedForStatus(status string, now time.Time) (*time.Time, *time.Time) {
	switch status {
	case "running":
		return &now, nil
	case "success", "failed", "canceled":
		return nil, &now
	default:
		return nil, nil
	}
}

func completedAtForStatus(status string, now time.Time) *time.Time {
	switch status {
	case "success", "failed", "canceled":
		return &now
	default:
		return nil
	}
}
