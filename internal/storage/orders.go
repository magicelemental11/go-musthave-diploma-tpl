package storage

import (
	"context"
	"errors"
	"time"
)

// ErrOrderOwnedByAnother is returned when the order number was uploaded by a
// different user.
var ErrOrderOwnedByAnother = errors.New("order belongs to another user")

// Order statuses.
const (
	StatusNew        = "NEW"
	StatusProcessing = "PROCESSING"
	StatusInvalid    = "INVALID"
	StatusProcessed  = "PROCESSED"
)

// Order is a row of the orders table. Accrual is in hundredths of a point and
// is nil until the order is processed.
type Order struct {
	Number     string
	Status     string
	Accrual    *int64
	UploadedAt time.Time
}

// AddOrder registers the order for the user. It returns created=false if the
// user has already uploaded it and ErrOrderOwnedByAnother if someone else has.
func (s *Storage) AddOrder(ctx context.Context, userID int64, number string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO orders (number, user_id) VALUES ($1, $2) ON CONFLICT (number) DO NOTHING`,
		number, userID,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil
	}
	var owner int64
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE number = $1`, number).Scan(&owner); err != nil {
		return false, err
	}
	if owner != userID {
		return false, ErrOrderOwnedByAnother
	}
	return false, nil
}

// ListOrders returns the user's orders, newest first.
func (s *Storage) ListOrders(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT number, status, accrual, uploaded_at FROM orders
		 WHERE user_id = $1 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
