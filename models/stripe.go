package models

import (
	"context"
	"fmt"
	"log"

	"github.com/arashthr/go-course/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v81"
)

type StripeService struct {
	Pool *pgxpool.Pool
}

func (s *StripeService) SaveSession(userId uint, sessionId string) error {
	log.Printf("save session: userId: %d, sessionId: %s\n", userId, sessionId)
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_sessions (user_id, stripe_session_id)
		VALUES ($1, $2);`, userId, sessionId)
	if err != nil {
		return fmt.Errorf("saving stripe session: %w", err)
	}
	return nil
}

func (s *StripeService) ProcessInvoice(invoice *stripe.Invoice) {
	// TODO: respond before processing
	if invoice.Subscription == nil {
		log.Printf("No subscription found in invoice")
		return
	}
	// Get id from

	// Insert customer into database
	s.Pool.Exec(context.Background(), `
		INSERT INTO customers (user_id, stripe_customer_id)
	`)
}

func (s *StripeService) GetCustomerId(userId uint) (customerId string, err error) {
	err = s.Pool.QueryRow(context.Background(), `
		SELECT stripe_customer_id FROM stripe_customers
		WHERE user_id = $1;
	`, userId).Scan(&customerId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNoStripeCustomer
		}
		return "", fmt.Errorf("getting stripe customer id: %w", err)
	}
	return customerId, nil
}

func (s *StripeService) InsertCustomerId(userId uint, customerId string) error {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_customers (user_id, stripe_customer_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET stripe_customer_id = $2;
	`, userId, customerId)
	return err
}
