// Package db provides database access to the Neon PostgreSQL database
// for querying the calendar_connect table.
package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a calendar_connect record does not exist for the given owner.
var ErrNotFound = errors.New("db: calendar connection not found")

// CalendarConnect represents a row from the public.calendar_connect table.
type CalendarConnect struct {
	OwnerID      string  `db:"owner_id"`
	Gmail        string  `db:"gmail"`
	RefreshToken string  `db:"refresh_token"`
	Scope        *string `db:"scope"` // nullable — nil means no scope granted
}

var (
	pool     *pgxpool.Pool
	poolOnce sync.Once
	poolErr  error
)

// getPool initializes and returns the shared connection pool.
// The pool is created once per Lambda execution context and reused across warm invocations.
func getPool(databaseURL string) (*pgxpool.Pool, error) {
	poolOnce.Do(func() {
		pool, poolErr = pgxpool.New(context.Background(), databaseURL)
	})
	return pool, poolErr
}

// FetchCalendarConnect queries the calendar_connect table by owner_id
// and returns the user's calendar connection details.
// Returns ErrNotFound if the user has no record; wraps other failures.
func FetchCalendarConnect(ctx context.Context, databaseURL string, ownerID string) (*CalendarConnect, error) {
	p, err := getPool(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: failed to initialize connection pool: %w", err)
	}

	var cc CalendarConnect
	err = p.QueryRow(ctx,
		`SELECT owner_id, gmail, refresh_token, scope
		 FROM public.calendar_connect
		 WHERE owner_id = $1
		 LIMIT 1`,
		ownerID,
	).Scan(&cc.OwnerID, &cc.Gmail, &cc.RefreshToken, &cc.Scope)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: query failed: %w", err)
	}

	return &cc, nil
}
