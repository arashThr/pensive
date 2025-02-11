package models

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StripeService struct {
	Pool *pgxpool.Pool
}

func (s *StripeService) SaveSession(userId uint, sessionId string, customerId string) error {
	return errors.New("not implemented")
}
