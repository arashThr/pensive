package models

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/arashthr/go-course/errors"
	"github.com/arashthr/go-course/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v81"
)

type StripeService struct {
	Pool *pgxpool.Pool
}

type Subscription struct {
	ID                 string
	UserID             types.UserId
	StripeCustomerID   string
	Status             string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
}

type SubscriptionHistory struct {
	ID                   int
	UserID               int
	StripeSubscriptionID string
	Status               string
	StartedAt            time.Time
	EndedAt              time.Time
}

func (s *StripeService) SaveSession(userId types.UserId, sessionId string) error {
	log.Printf("save session: userId: %d, sessionId: %s\n", userId, sessionId)
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_sessions (user_id, stripe_session_id)
		VALUES ($1, $2);`, userId, sessionId)
	if err != nil {
		return fmt.Errorf("saving stripe session: %w", err)
	}
	return nil
}

func (s *StripeService) HandleInvoicePaid(invoice *stripe.Invoice) {
	// TODO: respond before processing
	if invoice.Subscription == nil {
		slog.Error("No subscription found in invoice")
		return
	}
	if invoice.Status != stripe.InvoiceStatusPaid {
		slog.Info("Invoice not paid", "status", invoice.Status, "invoiceID", invoice.ID)
		return
	}
	customerId := invoice.Customer.ID
	userId, err := s.GetUserIdByStripeCustomerId(customerId)
	if err != nil {
		if errors.Is(err, ErrNoStripeCustomer) {
			slog.Error("We expect to have userId for the customer", "customerId", customerId)
			return
		}
		slog.Error("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	slog.Info("Processing invoice", "userId", userId, "invoiceID", invoice.ID)

	subscription := &Subscription{
		UserID:             userId,
		StripeCustomerID:   customerId,
		ID:                 invoice.Subscription.ID,
		Status:             "active",
		CurrentPeriodStart: time.Unix(invoice.PeriodStart, 0),
		CurrentPeriodEnd:   time.Unix(invoice.PeriodEnd, 0),
	}

	_, err = s.Pool.Exec(context.Background(), `
		INSERT INTO subscriptions (id, user_id, stripe_customer_id, status, current_period_start, current_period_end)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET status = $4, current_period_start = $5, current_period_end = $6`,
		subscription.ID, subscription.UserID, subscription.StripeCustomerID, subscription.Status,
		subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd)
	if err != nil {
		slog.Error("Error saving subscription", "error", err)
		return
	}

	// TODO: Add to payment history
	s.appendHistory(subscription)
	slog.Info("Saved subscription", "userId", userId, "subscriptionID", subscription.ID)
}

func (s *StripeService) HandleSubscriptionUpdated(subscription *stripe.Subscription) {
	// TODO: Add to subscription history
}

func (s *StripeService) HandleSubscriptionDeleted(subscription *stripe.Subscription) {
	// HERE
	_, err := s.Pool.Exec(context.Background(), `
	UPDATE subscriptions
	SET status = $1
	WHERE id = $2;`, subscription.Status, subscription.ID)

	if err != nil {
		slog.Error("Error updating subscription", "error", err)
		return
	}
	// TODO: Add to subscription history
	slog.Info("Subscription deleted", "subscriptionID", subscription.ID)
}

func (s *StripeService) GetUserIdByStripeCustomerId(customerId string) (userId types.UserId, err error) {
	err = s.Pool.QueryRow(context.Background(),
		`SELECT user_id FROM stripe_customers
		WHERE stripe_customer_id = $1;`, customerId).Scan(&userId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return userId, ErrNoStripeCustomer
		}
		return userId, fmt.Errorf("getting user id by stripe customer id: %w", err)
	}
	return userId, nil
}

func (s *StripeService) GetCustomerIdByUserId(userId types.UserId) (customerId string, err error) {
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

func (s *StripeService) InsertCustomerId(userId types.UserId, customerId string) error {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_customers (user_id, stripe_customer_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET stripe_customer_id = $2;
	`, userId, customerId)
	return err
}

func (s *StripeService) appendHistory(subscription *Subscription) {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO subscription_history (user_id, stripe_subscription_id, status, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5);`,
		subscription.UserID, subscription.ID, subscription.Status, subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd)
	if err != nil {
		slog.Error("Error saving subscription history", "error", err)
	}
}
