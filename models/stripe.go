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
	stripeclient "github.com/stripe/stripe-go/v81"
	subscriptionclient "github.com/stripe/stripe-go/v81/subscription"
)

type StripeService struct {
	Pool *pgxpool.Pool
}

type Subscription struct {
	UserID               types.UserId
	StripeSubscriptionID string
	Status               string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CanceledAt           time.Time
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

func (s *StripeService) HandleInvoicePaid(invoice *stripeclient.Invoice, rawEvent *json.RawMessage) {
	if invoice.Subscription == nil {
		slog.Error("No subscription found in invoice paid event. We do not expect to receive this", "invoiceID", invoice.ID)
		return
	}
	if invoice.Status != stripeclient.InvoiceStatusPaid {
		slog.Error("Invoice not paid", "status", invoice.Status, "invoiceID", invoice.ID)
		return
	}
	customerId := invoice.Customer.ID
	userId, err := s.GetUserIdByStripeCustomerId(customerId)
	if err != nil {
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
	tx.QueryRow(context.Background(), `
		SELECT * FROM invoices
		WHERE stripe_invoice_id = $1;`, invoice.ID).Scan(&prevInvoice)
	if prevInvoice != "" {
		slog.Info("Invoice already processed", "invoiceID", invoice.ID)
		return
	}

	// Get the full subscription details from Stripe
	// We need this because invoice doesn't contain all subscription details
	sub, err := subscriptionclient.Get(invoice.Subscription.ID, &stripeclient.SubscriptionParams{})
	if err != nil {
		slog.Error("Error getting subscription", "error", err, "subscriptionID", invoice.Subscription.ID)
		return
	}

	if sub.Status == "active" && invoice.BillingReason == "subscription_create" {
		slog.Info("First invoice for subscription", "subscriptionID", sub.ID)
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
		slog.Error("Error saving invoice", "error", err, "invoiceID", invoice.ID, "userId", userId)
		return
	}

	// Insert subscription into subscriptions table
	newSub := Subscription{
		UserID:               userId,
		StripeSubscriptionID: sub.ID,
		Status:               string(sub.Status),
		CurrentPeriodStart:   time.Unix(sub.CurrentPeriodStart, 0),
		CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
		CanceledAt:           time.Unix(sub.CanceledAt, 0),
	}
	fmt.Println("newSub", newSub)
	_, err = tx.Exec(context.Background(), `
		INSERT INTO subscriptions (stripe_subscription_id, user_id, status, current_period_start, current_period_end, canceled_at)
		VALUES ($1, $2, $3, $4, $5, $6);`,
		newSub.StripeSubscriptionID, newSub.UserID, newSub.Status,
		newSub.CurrentPeriodStart, newSub.CurrentPeriodEnd, newSub.CanceledAt)
	if err != nil {
		slog.Error("Error saving subscription", "error", err, "subscriptionID", newSub.ID)
		return
	}

	// Update the subscription status in users table
	_, err = tx.Exec(context.Background(), `
		UPDATE users
		SET subscription_status = $1
		WHERE id = $2;`, "premium", userId)
	if err != nil {
		slog.Error("Error updating subscription", "error", err)
		return
	}

	// Log the event
	event := &SubscriptionEvent{
		SubscriptionID: sub.ID,
		EventType:      "invoice.paid",
		EventData:      *rawEvent,
	}
	err = recordEvent(tx, event)
	if err != nil {
		slog.Error("Error saving subscription event for paid invoice", "error", err, "subscriptionID", sub.ID)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		slog.Error("Error committing transaction", "error", err)
		return
	}

	slog.Info("Invoice processed", "invoiceID", invoice.ID, "userId", userId)
}

func (s *StripeService) HandleInvoiceFailed(invoice *stripeclient.Invoice, rawEvent *json.RawMessage) {
	if invoice.Subscription == nil {
		slog.Error("No subscription found in invoice failed event. We do not expect to receive this", "invoiceID", invoice.ID)
		return
	}

	customerId := invoice.Customer.ID
	userId, err := s.GetUserIdByStripeCustomerId(customerId)
	if err != nil {
		slog.Error("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	slog.Info("Processing invoice failed", "userId", userId, "invoiceID", invoice.ID)

	// Get the full subscription details from Stripe
	// We need this because invoice doesn't contain all subscription
	sub, err := subscriptionclient.Get(invoice.Subscription.ID, &stripeclient.SubscriptionParams{})
	if err != nil {
		slog.Error("Error getting subscription", "error", err, "subscriptionID", invoice.Subscription.ID)
		return
	}

	newInvoice := &StripeInvoice{
		StripeInvoiceId: invoice.ID,
		SubscriptionID:  invoice.Subscription.ID,
		Status:          string(invoice.Status),
		Amount:          invoice.AmountPaid,
		PaidAt:          time.Unix(invoice.StatusTransitions.PaidAt, 0),
		CreatedAt:       time.Unix(invoice.Created, 0),
	}

	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		slog.Error("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Insert invoice into db
	_, err = tx.Exec(context.Background(), `
		INSERT INTO invoices (stripe_invoice_id, stripe_subscription_id, status, amount, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6);`, newInvoice.StripeInvoiceId, newInvoice.SubscriptionID,
		newInvoice.Status, newInvoice.Amount, newInvoice.PaidAt, newInvoice.CreatedAt)
	if err != nil {
		slog.Error("Error saving invoice", "error", err, "invoiceID", invoice.ID, "userId", userId)
		return
	}

	// HERE
	// Update the subscription status
	// TODO: Do I want to do it for failed payments? Or is it better to do it when subscription deleted
	// _, err = tx.Exec(context.Background(), `
	// 	UPDATE subscriptions
	// 	SET status = $1
	// 	WHERE stripe_subscription_id = $2;`, sub.Status, sub.ID)
	// if err != nil {
	// 	slog.Error("Error updating subscription", "error", err, "subscriptionID", sub.ID)
	// 	return
	// }

	// Log the event
	event := &SubscriptionEvent{
		SubscriptionID: sub.ID,
		EventType:      "invoice.payment_failed",
		EventData:      *rawEvent,
	}
	err = recordEvent(tx, event)
	if err != nil {
		slog.Error("Error saving subscription event for failed invoice", "error", err, "subscriptionID", sub.ID)
		return
	}

	// TODO: Notify user that the payment has failed

	err = tx.Commit(context.Background())
	if err != nil {
		slog.Error("Error committing transaction", "error", err)
		return
	}

	slog.Info("Invoice failed processed", "invoiceID", invoice.ID, "userId", userId)
}

func (s *StripeService) HandleSubscriptionCreated(subscription *stripeclient.Subscription, rawEvent *json.RawMessage) {
	// Get user ID
	customerId := subscription.Customer.ID
	userId, err := s.GetUserIdByStripeCustomerId(customerId)
	if err != nil {
		slog.Error("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	slog.Info("Processing subscription deletion", "userId", userId, "subscriptionID", subscription.ID)

	// Start transaction
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		slog.Error("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

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
		VALUES ($1, $2, $3, $4, $5, $6);`,
		sub.StripeSubscriptionID, sub.UserID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt)
	if err != nil {
		slog.Error("Error saving subscription", "error", err, "subscriptionID", subscription.ID)
		return
	}

	// Record raw event in db
	event := &SubscriptionEvent{
		SubscriptionID: subscription.ID,
		EventType:      "customer.subscription.created",
		EventData:      *rawEvent,
	}
	err = recordEvent(tx, event)
	if err != nil {
		slog.Error("Error saving subscription event for failed invoice", "error", err, "subscriptionID", subscription.ID)
		return
	}

	// Finalize queries
	err = tx.Commit(context.Background())
	if err != nil {
		slog.Error("Error committing transaction", "error", err)
		return
	}

	slog.Info("Subscription created", "subscriptionID", subscription.ID)
}

func (s *StripeService) HandleSubscriptionDeleted(subscription *stripeclient.Subscription, rawEvent *json.RawMessage) {
	// Get user Id
	customerId := subscription.Customer.ID
	userId, err := s.GetUserIdByStripeCustomerId(customerId)
	if err != nil {
		slog.Error("Error getting user id", "customerId", customerId, "error", err)
		return
	}
	slog.Info("Processing subscription deletion", "userId", userId, "subscriptionID", subscription.ID)

	// Start tx
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		slog.Error("Error starting transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Update the subscription status
	_, err = tx.Exec(context.Background(), `
		UPDATE users
		SET subscription_status = $1
		WHERE id = $2;`, "canceled", userId)
	if err != nil {
		slog.Error("Error updating subscription", "error", err)
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
		slog.Error("Error saving subscription", "error", err, "subscriptionID", subscription.ID)
		return
	}

	// Record raw event in db
	event := &SubscriptionEvent{
		SubscriptionID: subscription.ID,
		EventType:      "customer.subscription.deleted",
		EventData:      *rawEvent,
	}
	err = recordEvent(tx, event)
	if err != nil {
		slog.Error("Error saving subscription event for failed invoice", "error", err, "subscriptionID", subscription.ID)
		return
	}

	err = tx.Commit(context.Background())
	if err != nil {
		slog.Error("Error committing transaction", "error", err)
		return
	}

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

func recordEvent(tx pgx.Tx, event *SubscriptionEvent) error {
	_, err := tx.Exec(context.Background(), `
	INSERT INTO subscription_events (stripe_subscription_id, event_type, event_data)
	VALUES ($1, $2, $3);`,
		event.SubscriptionID, event.EventType, event.EventData)
	if err != nil {
		return fmt.Errorf("saving subscription event: %w", err)
	}
	return nil
}
