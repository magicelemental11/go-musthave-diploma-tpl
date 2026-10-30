package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrLoginTaken is returned when a user with the same login already exists.
	ErrLoginTaken = errors.New("login already taken")
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("not found")
)

// User is a row of the users table.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

// CreateUser inserts a user and returns its ID. It returns ErrLoginTaken if
// the login is already registered.
func (s *Storage) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, ErrLoginTaken
		}
		return 0, err
	}
	return id, nil
}

// GetUserByLogin returns the user with the given login or ErrNotFound.
func (s *Storage) GetUserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash FROM users WHERE login = $1`,
		login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}
