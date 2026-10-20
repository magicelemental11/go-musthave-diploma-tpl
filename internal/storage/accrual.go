package storage

import (
	"context"
	"database/sql"
	"errors"
)

// ListPending returns up to limit numbers of orders that are still NEW or
// PROCESSING and sort after the given number, in ascending order. Pass the
// last returned number as after to fetch the next page.
func (s *Storage) ListPending(ctx context.Context, after string, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT number FROM orders WHERE status IN ('NEW', 'PROCESSING') AND number > $1
		 ORDER BY number LIMIT $2`,
		after, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var numbers []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		numbers = append(numbers, n)
	}
	return numbers, rows.Err()
}

// ApplyAccrual sets the order's status and accrual and, when the order becomes
// PROCESSED with a positive accrual, credits it to the owner's balance. Orders
// that are already final are left untouched, so a repeated call credits
// nothing. Accrual is in hundredths of a point.
func (s *Storage) ApplyAccrual(ctx context.Context, number, status string, accrual *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`UPDATE orders SET status = $2, accrual = $3
		 WHERE number = $1 AND status NOT IN ('PROCESSED', 'INVALID') AND status <> $2
		 RETURNING user_id`,
		number, status, accrual,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	if status == StatusProcessed && accrual != nil && *accrual > 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET balance = balance + $1 WHERE id = $2`,
			*accrual, userID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
