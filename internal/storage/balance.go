package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrInsufficientFunds is returned when a withdrawal exceeds the balance.
var ErrInsufficientFunds = errors.New("insufficient funds")

// Withdrawal is a row of the withdrawals table. Sum is in hundredths of a
// point.
type Withdrawal struct {
	Order       string
	Sum         int64
	ProcessedAt time.Time
}

// GetBalance returns the current balance and the total withdrawn amount in
// hundredths of a point, or ErrNotFound if the user does not exist.
func (s *Storage) GetBalance(ctx context.Context, userID int64) (current, withdrawn int64, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT balance, withdrawn FROM users WHERE id = $1`,
		userID,
	).Scan(&current, &withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	return current, withdrawn, err
}

// Withdraw takes sum from the user's balance and records the withdrawal for
// order. It returns ErrInsufficientFunds if the balance is too low.
func (s *Storage) Withdraw(ctx context.Context, userID int64, order string, sum int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE users SET balance = balance - $1, withdrawn = withdrawn + $1
		 WHERE id = $2 AND balance >= $1`,
		sum, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInsufficientFunds
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`,
		userID, order, sum,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ListWithdrawals returns the user's withdrawals, newest first.
func (s *Storage) ListWithdrawals(ctx context.Context, userID int64) ([]Withdrawal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT order_number, sum, processed_at FROM withdrawals
		 WHERE user_id = $1 ORDER BY processed_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Withdrawal
	for rows.Next() {
		var w Withdrawal
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}
