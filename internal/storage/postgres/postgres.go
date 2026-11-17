// Package postgres is a PostgreSQL storage.Storage; schema migrations run on New.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matchbull6/go_url_shortener/internal/storage"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dbURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: migrate: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Get(ctx context.Context, code string) (storage.Link, error) {
	var link storage.Link
	err := s.pool.QueryRow(ctx,
		`SELECT code, url, clicks, created_at FROM links WHERE code = $1`, code).
		Scan(&link.Code, &link.URL, &link.Clicks, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.Link{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.Link{}, fmt.Errorf("postgres: get %q: %w", code, err)
	}
	return link, nil
}

func (s *Store) Visit(ctx context.Context, code string) (string, error) {
	var url string
	err := s.pool.QueryRow(ctx,
		`UPDATE links SET clicks = clicks + 1 WHERE code = $1 RETURNING url`, code).
		Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", storage.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("postgres: visit %q: %w", code, err)
	}
	return url, nil
}

func (s *Store) Save(ctx context.Context, code, url string) error {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO links (code, url) VALUES ($1, $2) ON CONFLICT (code) DO NOTHING`,
		code, url)
	if err != nil {
		return fmt.Errorf("postgres: save %q: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrExists
	}
	return nil
}

func (s *Store) Close() {
	s.pool.Close()
}
