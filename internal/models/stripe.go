package models

import (
	"context"
	"fmt"
	"time"

	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	stripeclient "github.com/stripe/stripe-go/v81"
)

type StripeModel struct {
	Pool *pgxpool.Pool
}

func NewStripeModel(stripKey string, pool *pgxpool.Pool) *StripeModel {
	// Initialize stripe key
	stripeclient.Key = stripKey
	return &StripeModel{
		Pool: pool,
	}
}

type Subscription struct {
	UserID               types.UserId
	StripeSubscriptionID string
	Status               string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CanceledAt           time.Time
	PreviousAttributes   map[string]any
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

func (s *StripeModel) SaveSession(userId types.UserId, sessionId string) error {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_sessions (user_id, stripe_session_id)
		VALUES ($1, $2);`, userId, sessionId)
	if err != nil {
		return fmt.Errorf("saving stripe session: %w", err)
	}
	return nil
}

func (s *StripeModel) HandleInvoicePaid(invoice *stripeclient.Invoice) {
	if invoice.Subscription == nil {
		logging.Logger.Errorw("No subscription found in invoice paid event. We do not expect to receive this", "invoiceID", invoice.ID)
		return
	}
	if invoice.Status != stripeclient.InvoiceStatusPaid {
		logging.Logger.Errorw("Invoice not paid", "status", invoice.Status, "invoiceID", invoice.ID)
		return
	}
	customerId := invoice.Customer.ID
	userId, err := s.getUserIdByStripeCustomerId(customerId)
	if err != nil {
		logging.Logger.Errorw("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	logging.Logger.Infow("Processing invoice", "userId", userId, "invoiceID", invoice.ID)

	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Check if we already processed this invoice (idempotency)
	var prevInvoice string
	tx.QueryRow(context.Background(), `
		SELECT * FROM invoices
		WHERE stripe_invoice_id = $1;`, invoice.ID).Scan(&prevInvoice)
	if prevInvoice != "" {
		logging.Logger.Infow("Invoice already processed", "invoiceID", invoice.ID)
		return
	}

	newInvoice := &StripeInvoice{
		StripeInvoiceId: invoice.ID,
		SubscriptionID:  invoice.Subscription.ID,
		Status:          string(invoice.Status),
		Amount:          invoice.AmountPaid,
		PaidAt:          time.Unix(invoice.StatusTransitions.PaidAt, 0),
	}

	_, err = tx.Exec(context.Background(), `
		INSERT INTO invoices (stripe_invoice_id, stripe_subscription_id, status, amount, paid_at)
		VALUES ($1, $2, $3, $4, $5);`,
		newInvoice.StripeInvoiceId, newInvoice.SubscriptionID, newInvoice.Status,
		newInvoice.Amount, newInvoice.PaidAt)
	if err != nil {
		logging.Logger.Errorw("Error saving invoice", "error", err, "invoiceID", invoice.ID, "userId", userId)
		return
	}

	// Update the subscription status in users table
	_, err = tx.Exec(context.Background(), `
		UPDATE users
		SET subscription_status = $1,
		stripe_invoice_id = $2
		WHERE id = $3;`, "premium", invoice.ID, userId)
	if err != nil {
		logging.Logger.Errorw("Error updating user subscription status", "error", err)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error committing transaction", "error", err)
		return
	}

	logging.Logger.Infow("Invoice processed", "invoiceID", invoice.ID, "userId", userId)
}

func (s *StripeModel) HandleInvoiceFailed(invoice *stripeclient.Invoice) {
	if invoice.Subscription == nil {
		logging.Logger.Errorw("No subscription found in invoice failed event. We do not expect to receive this", "invoiceID", invoice.ID)
		return
	}

	customerId := invoice.Customer.ID
	userId, err := s.getUserIdByStripeCustomerId(customerId)
	if err != nil {
		logging.Logger.Errorw("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	logging.Logger.Infow("Processing invoice failed", "userId", userId, "invoiceID", invoice.ID)

	newInvoice := &StripeInvoice{
		StripeInvoiceId: invoice.ID,
		SubscriptionID:  invoice.Subscription.ID,
		Status:          string(invoice.Status),
		Amount:          invoice.AmountPaid,
		PaidAt:          time.Unix(invoice.StatusTransitions.PaidAt, 0),
		CreatedAt:       time.Unix(invoice.Created, 0),
	}

	// Start tx
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// TODO: Notify user that the payment has failed

	// Insert invoice into db
	_, err = tx.Exec(context.Background(), `
		INSERT INTO invoices (stripe_invoice_id, stripe_subscription_id, status, amount, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6);`, newInvoice.StripeInvoiceId, newInvoice.SubscriptionID,
		newInvoice.Status, newInvoice.Amount, newInvoice.PaidAt, newInvoice.CreatedAt)
	if err != nil {
		logging.Logger.Errorw("Error saving invoice", "error", err, "invoiceID", invoice.ID, "userId", userId)
		return
	}

	_, err = tx.Exec(context.Background(), `
		UPDATE users
		SET stripe_invoice_id = $1
		WHERE id = $2;`, invoice.ID, userId)
	if err != nil {
		logging.Logger.Errorw("Error updating user for failed invoice", "error", err)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error committing transaction", "error", err)
		return
	}

	logging.Logger.Infow("Invoice failed processed", "invoiceID", invoice.ID, "userId", userId)
}

func (s *StripeModel) RecordSubscription(subscription *stripeclient.Subscription, prevAttr map[string]any) {
	// Get user ID
	customerId := subscription.Customer.ID
	userId, err := s.getUserIdByStripeCustomerId(customerId)
	if err != nil {
		logging.Logger.Errorw("Error getting user id for record subscription", "customerId", customerId, "error", err)
		return
	}
	logging.Logger.Infow("Processing subscription", "userId", userId, "subscriptionID", subscription.ID)

	// Insert subscription into subscriptions table
	sub := Subscription{
		UserID:               userId,
		StripeSubscriptionID: subscription.ID,
		Status:               string(subscription.Status),
		CurrentPeriodStart:   time.Unix(subscription.CurrentPeriodStart, 0),
		CurrentPeriodEnd:     time.Unix(subscription.CurrentPeriodEnd, 0),
		CanceledAt:           time.Unix(subscription.CanceledAt, 0),
		PreviousAttributes:   prevAttr,
	}
	_, err = s.Pool.Exec(context.Background(), `
		INSERT INTO subscriptions (stripe_subscription_id, user_id, status, current_period_start, current_period_end, canceled_at, previous_attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7);`, sub.StripeSubscriptionID, sub.UserID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt, sub.PreviousAttributes)
	if err != nil {
		logging.Logger.Errorw("Error saving subscription", "error", err, "subscriptionID", subscription.ID)
		return
	}

	logging.Logger.Infow("Subscription inserted", "subscriptionID", subscription.ID)
}

func (s *StripeModel) HandleSubscriptionDeleted(subscription *stripeclient.Subscription) {
	// Get user Id
	customerId := subscription.Customer.ID
	userId, err := s.getUserIdByStripeCustomerId(customerId)
	if err != nil {
		logging.Logger.Errorw("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	logging.Logger.Infow("Processing subscription deletion", "userId", userId, "subscriptionID", subscription.ID)

	// Start tx
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Update the subscription status
	_, err = tx.Exec(context.Background(), `
		UPDATE users
		SET subscription_status = $1
		WHERE id = $2;`, "canceled", userId)
	if err != nil {
		logging.Logger.Errorw("Error updating subscription", "error", err)
		return
	}

	// Insert subscription into subscriptions table
	sub := Subscription{
		UserID:               userId,
		StripeSubscriptionID: subscription.ID,
		Status:               string(subscription.Status),
		CurrentPeriodStart:   time.Unix(subscription.CurrentPeriodStart, 0),
		CurrentPeriodEnd:     time.Unix(subscription.CurrentPeriodEnd, 0),
		CanceledAt:           time.Unix(subscription.CanceledAt, 0),
	}
	_, err = tx.Exec(context.Background(), `
		INSERT INTO subscriptions (stripe_subscription_id, user_id, status, current_period_start, current_period_end, canceled_at)
		VALUES ($1, $2, $3, $4, $5, $6);`, sub.StripeSubscriptionID, sub.UserID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt)
	if err != nil {
		logging.Logger.Errorw("Error saving subscription", "error", err, "subscriptionID", subscription.ID)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		logging.Logger.Errorw("Error committing transaction", "error", err)
		return
	}

	logging.Logger.Infow("Subscription deleted", "subscriptionID", subscription.ID)
}

func (s *StripeModel) GetCustomerIdByUserId(userId types.UserId) (customerId string, err error) {
	err = s.Pool.QueryRow(context.Background(), `
		SELECT stripe_customer_id FROM stripe_customers
		WHERE user_id = $1;
	`, userId).Scan(&customerId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.ErrNoStripeCustomer
		}
		return "", fmt.Errorf("getting stripe customer id: %w", err)
	}
	return customerId, nil
}

func (s *StripeModel) InsertCustomerId(userId types.UserId, customerId string) error {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO stripe_customers (user_id, stripe_customer_id)
		VALUES ($1, $2);`, userId, customerId)
	return err
}

func (s *StripeModel) getUserIdByStripeCustomerId(customerId string) (userId types.UserId, err error) {
	err = s.Pool.QueryRow(context.Background(),
		`SELECT user_id FROM stripe_customers
		WHERE stripe_customer_id = $1;`, customerId).Scan(&userId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return userId, errors.ErrNoStripeCustomer
		}
		return userId, fmt.Errorf("getting user id by stripe customer id: %w", err)
	}
	return userId, nil
}

// func recordEvent(tx pgx.Tx, event *SubscriptionEvent) error {
// 	_, err := tx.Exec(context.Background(), `
// 	INSERT INTO subscription_events (stripe_subscription_id, event_type, event_data)
// 	VALUES ($1, $2, $3);`,
// 		event.SubscriptionID, event.EventType, event.EventData)
// 	if err != nil {
// 		return fmt.Errorf("saving subscription event: %w", err)
// 	}
// 	return nil
// }
