package postgres

import "context"

func (s *Store) Truncate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE links`)
	return err
}
