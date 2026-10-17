package authentication

import (
	"errors"
	"fmt"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func postgresExpiryTime(now time.Time, d time.Duration) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: now.Add(d), Valid: true}
}

func normalizeEmail(userEmail string) (string, error) {
	normalized := email.Normalize(userEmail)
	if _, err := email.ParseAddress(normalized); err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidEmail, userEmail)
	}
	return normalized, nil
}

// Reports whether err is a PostgreSQL unique constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
