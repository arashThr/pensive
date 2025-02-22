package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/arashthr/go-course/errors"
	"github.com/arashthr/go-course/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	stripeClient "github.com/stripe/stripe-go/v81"
	subscriptionClient "github.com/stripe/stripe-go/v81/subscription"
)

type StripeService struct {
	Pool *pgxpool.Pool
}

type Subscription struct {
	UserID               types.UserId
	StripeSubscriptionID string
	StripePriceID        string
	Status               string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
}

type StripeInvoice struct {
	StripeInvoiceId string
	SubscriptionID  string
	Status          string
	Amount          int64
	PaidAt          time.Time
	CreatedAt       time.Time
}

type SubscriptionEvent struct {
	SubscriptionID string
	EventType      string
	EventData      []byte
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

func (s *StripeService) HandleInvoicePaid(invoice *stripeClient.Invoice, rawEvent *json.RawMessage) {
	if invoice.Subscription == nil {
		slog.Error("No subscription found in invoice. We do not expect to receive this", "invoiceID", invoice.ID)
		return
	}
	if invoice.Status != stripeClient.InvoiceStatusPaid {
		slog.Error("Invoice not paid", "status", invoice.Status, "invoiceID", invoice.ID)
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

	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		slog.Error("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Check if we already processed this invoice (idempotency)
	var prevInvoice string
	s.Pool.QueryRow(context.Background(), `
		SELECT * FROM invoices
		WHERE id = $1;`, invoice.ID).Scan(&prevInvoice)
	if prevInvoice != "" {
		slog.Info("Invoice already processed", "invoiceID", invoice.ID)
		return
	}

	// Get the full subscription details from Stripe
	// We need this because invoice doesn't contain all subscription details
	sub, err := subscriptionClient.Get(invoice.Subscription.ID, &stripeClient.SubscriptionParams{})
	if err != nil {
		slog.Error("Error getting subscription", "error", err, "subscriptionID", invoice.Subscription.ID)
		return
	}

	isFirstInvoice := false
	if sub.Status == "active" && invoice.BillingReason == "subscription_create" {
		isFirstInvoice = true
	}

	if isFirstInvoice {
		newSub := &Subscription{
			UserID:               userId,
			StripeSubscriptionID: invoice.Subscription.ID,
			Status:               "active",
			CurrentPeriodStart:   time.Unix(sub.CurrentPeriodStart, 0),
			CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
		}

		_, err := s.Pool.Exec(context.Background(), `
			INSERT INTO subscriptions (user_id, stripe_subscription_id, status, current_period_start, current_period_end)
			VALUES ($1, $2, $3, $4, $5, $6);`,
			newSub.UserID, newSub.StripeSubscriptionID, newSub.Status,
			newSub.CurrentPeriodStart, newSub.CurrentPeriodEnd)
		if err != nil {
			slog.Error("Error saving new subscription", "error", err, "invoiceID", invoice.ID, "userId", userId)
			return
		}
	}

	newInvoice := &StripeInvoice{
		StripeInvoiceId: invoice.ID,
		SubscriptionID:  invoice.Subscription.ID,
		Status:          string(invoice.Status),
		Amount:          invoice.AmountPaid,
		PaidAt:          time.Unix(invoice.StatusTransitions.PaidAt, 0),
	}

	_, err = s.Pool.Exec(context.Background(), `
		INSERT INTO invoices (stripe_invoice_id, subscription_id, status, amount, paid_at)
		VALUES ($1, $2, $3, $4, $5);`,
		newInvoice.StripeInvoiceId, newInvoice.SubscriptionID, newInvoice.Status,
		newInvoice.Amount, newInvoice.PaidAt)
	if err != nil {
		slog.Error("Error saving invoice", "error", err, "invoiceID", invoice.ID, "userId", userId)
		return
	}

	_, err = s.Pool.Exec(context.Background(), `
		UPDATE subscriptions
		SET status = $1
		WHERE stripe_subscription_id = $2;`, sub.Status, sub.ID)
	if err != nil {
		slog.Error("Error updating subscription", "error", err, "subscriptionID", sub.ID)
		return
	}

	// Log the event
	event := &SubscriptionEvent{
		SubscriptionID: sub.ID,
		EventType:      "invoice.paid",
		EventData:      *rawEvent,
	}
	_, err = s.Pool.Exec(context.Background(), `
		INSERT INTO subscription_events (subscription_id, event_type, event_data)
		VALUES ($1, $2, $3);`,
		event.SubscriptionID, event.EventType, event.EventData)
	if err != nil {
		slog.Error("Error saving subscription event", "error", err, "subscriptionID", sub.ID)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		slog.Error("Error committing transaction", "error", err)
		return
	}

	slog.Info("Invoice processed", "invoiceID", invoice.ID, "userId", userId)
}

func (s *StripeService) HandleInvoiceFailed(invoice *stripeClient.Invoice) {
}

func (s *StripeService) HandleSubscriptionUpdated(subscription *stripeClient.Subscription) {
	// TODO: Add to subscription history
}

func (s *StripeService) HandleSubscriptionDeleted(subscription *stripeClient.Subscription) {
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
