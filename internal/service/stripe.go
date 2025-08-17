package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/web"
	stripeclient "github.com/stripe/stripe-go/v81"
	portalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/webhook"
)

type Stripe struct {
	Templates struct {
		Success web.Template
		Cancel  web.Template
	}
	Domain              string
	PriceId             string
	StripeWebhookSecret string
	StripeModel         *models.StripeModel
}

func (s Stripe) getStripeCustomerId(ctx context.Context, user *models.User) (customerId string, err error) {
	logger := loggercontext.Logger(ctx)
	customerId, err = s.StripeModel.GetCustomerIdByUserId(user.ID)
	if err == nil {
		return customerId, nil
	}
	if !errors.Is(err, errors.ErrNoStripeCustomer) {
		return "", fmt.Errorf("get stripe customer id: %w", err)
	}
	logger.Errorw("No stripe customer found for user %v", "error", err, user.ID)
	params := &stripeclient.CustomerListParams{Email: stripeclient.String(user.Email)}
	params.Filters.AddFilter("limit", "", "1")
	result := customer.List(params)
	if result.Next() {
		// Found customer
		logger.Errorw("Found stripe customer for user %v", "error", err, user.ID)
		customer := result.Customer()
		customerId = customer.ID
	} else {
		// Create a new customer
		params := &stripeclient.CustomerParams{Email: stripeclient.String(user.Email)}
		params.AddMetadata("user_id", strconv.Itoa(int(user.ID)))
		// TODO: params.SetIdempotencyKey()
		customer, err := customer.New(params)
		if err != nil {
			if stripeErr, ok := err.(*stripeclient.Error); ok {
				logger.Errorw("Stripe error: %v", "error", err, stripeErr.Error())
			} else {
				logger.Errorw("Create stripe customer error: %v", "error", err, err)
			}
			return "", err
		}
		logger.Errorw("Created stripe customer for user %v with customer id %v", "error", err, user.ID, customer.ID)
		customerId = customer.ID
	}
	s.StripeModel.InsertCustomerId(user.ID, customerId)
	return customerId, nil
}

func (s Stripe) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(r.Context())
	logger.Infow("create checkout session for user", "user_id", user.ID)
	customerId, err := s.getStripeCustomerId(ctx, user)
	if err != nil {
		logger.Errorw("get stripe customer id", "error", err, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	checkoutParams := &stripeclient.CheckoutSessionParams{
		Mode: stripeclient.String(string(stripeclient.CheckoutSessionModeSubscription)),
		LineItems: []*stripeclient.CheckoutSessionLineItemParams{
			{
				Price:    stripeclient.String(s.PriceId),
				Quantity: stripeclient.Int64(1),
			},
		},
		SuccessURL:          stripeclient.String(s.Domain + "/payments/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:           stripeclient.String(s.Domain + "/payments/cancel"),
		Customer:            &customerId,
		AllowPromotionCodes: stripeclient.Bool(true),
	}

	sess, err := session.New(checkoutParams)
	if err != nil {
		logger.Errorw("session new", "error", err, "user_id", user.ID)
	}

	// TODO: Save session and customer ID in db
	// err = s.StripeService.SaveSession(user.ID, sess.ID)
	// if err != nil {
	// 	log.Printf("s.StripeService.SaveSession: %v", err)
	// 	http.Error(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }
	logger.Errorw("checkout session", "error", err, "session_id", sess.ID)
	http.Redirect(w, r, sess.URL, http.StatusSeeOther)
}

func (s Stripe) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	logger.Infow("create portal session for user", "user_id", user.ID)
	// For demonstration purposes, we're using the Checkout session to retrieve the customer ID.
	// Typically this is stored alongside the authenticated user in your database.
	// TODO: Read session ID from db
	sessionId := r.FormValue("session_id")
	logger.Infow("create portal session", "session_id", sessionId)
	sess, err := session.Get(sessionId, nil)

	if err != nil {
		logger.Errorw("session get", "error", err, "session_id", sessionId, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	params := &stripeclient.BillingPortalSessionParams{
		Customer:  stripeclient.String(sess.Customer.ID),
		ReturnURL: stripeclient.String(s.Domain + "/users/me"),
	}
	ps, err := portalsession.New(params)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.Errorw("create portal session", "error", err)
		return
	}

	http.Redirect(w, r, ps.URL, http.StatusSeeOther)
}

func (s Stripe) GoToBillingPortal(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	customerId, err := s.StripeModel.GetCustomerIdByUserId(user.ID)
	if err != nil {
		logger.Errorw("get stripe customer id", "error", err, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	returnPath, err := url.JoinPath(s.Domain, "/users/me")
	if err != nil {
		logger.Errorw("url.JoinPath", "error", err, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	params := &stripeclient.BillingPortalSessionParams{
		Customer:  stripeclient.String(customerId),
		ReturnURL: stripeclient.String(returnPath),
	}

	ps, err := portalsession.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.Errorw("Go to billing portal", "error", err, "user_id", user.ID)
		return
	}

	http.Redirect(w, r, ps.URL, http.StatusSeeOther)
}

func (s Stripe) Success(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	logger.Infow("user completed the checkout session", "user_id", user.ID)
	// TODO: Should validate subscription here?
	var data struct {
		Title     string
		SessionId string
	}
	data.Title = "Payment Successful"
	data.SessionId = r.URL.Query().Get("session_id")
	s.Templates.Success.Execute(w, r, data)
}

func (s Stripe) Cancel(w http.ResponseWriter, r *http.Request) {
	// TODO: Keep track of payment statuses?
	var data struct {
		Title string
	}
	data.Title = "Payment Cancelled"
	s.Templates.Cancel.Execute(w, r, data)
}

func (s Stripe) Webhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	const MaxBodyBytes = int64(65536)
	bodyReader := http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(bodyReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	// Replace this endpoint secret with your endpoint's unique secret
	// If you are testing with the CLI, find the secret by running 'stripe listen'
	// If you are using an endpoint defined with the API or dashboard, look in your webhook settings
	// at https://dashboard.stripe.com/webhooks
	endpointSecret := s.StripeWebhookSecret
	signatureHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Webhook signature verification failed. %v\n", err)
		w.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
		return
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	logger.Infow("received webhook", "event_type", event.Type)
	switch event.Type {
	case "customer.subscription.deleted":
		subscription, err := getSubscriptionDeleted(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		go s.StripeModel.HandleSubscriptionDeleted(subscription)
	case "customer.subscription.updated":
		sub, err := handleSubscriptionUpdated(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// 1: record transition updates
		// 2: update prevAtt="map[cancel_at:<nil> cancel_at_period_end:false canceled_at:<nil> cancellation_details:map[reason:<nil>]]"
		// 3: feedback prevAtt=map[cancellation_details:map[feedback:<nil>]]
		go s.StripeModel.RecordSubscription(sub, event.Data.PreviousAttributes)
	case "customer.subscription.created":
		subscription, err := getSubscriptionCreated(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		go s.StripeModel.RecordSubscription(subscription, nil)
	case "customer.subscription.trial_will_end":
		// handleSubscriptionTrialWillEnd(subscription)
	case "entitlements.active_entitlement_summary.updated":
		logger.Infow("Active entitlement summary updated", "event_id", event.ID)
		// Then define and call a func to handle active entitlement summary updated.
		// handleEntitlementUpdated(subscription)
	case "checkout.session.completed":
		// Payment is successful and the subscription is created.
		// You should provision the subscription and save the customer ID to your database.
		err := handleCheckoutSessionCompleted(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case "invoice.paid":
		// Continue to provision the subscription as payments continue to be made.
		// Store the status in your database and check when a user accesses your service.
		// This approach helps you avoid hitting rate limits.
		// Parse the event data to get the subscription details
		invoice, err := getInvoicePaid(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		go s.StripeModel.HandleInvoicePaid(invoice)
	case "invoice.payment_failed":
		// The payment failed or the customer does not have a valid payment method.
		// The subscription becomes past_due. Notify your customer and send them to the
		// customer portal to update their payment information.
		invoice, err := getInvoiceFailed(ctx, &event)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		go s.StripeModel.HandleInvoiceFailed(invoice)
	default:
		logger.Warnw("Unhandled event type", "event_type", event.Type)
	}
	w.WriteHeader(http.StatusOK)
}

// FAILURE
/*
func handlePaymentFailure(invoice *stripe.Invoice) {
    // Here, you might:
    // - Notify the user about the failed payment
    // - Implement a retry mechanism if possible (Stripe does this automatically to some extent)
    // - Prepare to downgrade or limit access if this isn't the first failure

    if invoice.Subscription != nil {
        // Add logic to notify user or update their status if necessary
        notifyUserOfPaymentIssue(*invoice.Subscription.ID)
    }
}

func handleSubscriptionDeletion(subscription *stripe.Subscription) {
    // Downgrade user to free tier or remove premium access
    downgradeUserFromPremium(subscription.Customer.ID)
}
*/

func handleSubscriptionUpdated(ctx context.Context, event *stripeclient.Event) (*stripeclient.Subscription, error) {
	logger := loggercontext.Logger(ctx)
	var subscription stripeclient.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return nil, fmt.Errorf("parsing webhook JSON: %w", err)
	}
	logger.Infow("Subscription updated", "subscription_id", subscription.ID, "prev_att", event.Data.PreviousAttributes)
	if event.Data.PreviousAttributes["status"] != nil {
		logger.Infow("Subscription status changed", "prev_status", event.Data.PreviousAttributes["status"], "new_status", subscription.Status)
	}
	return &subscription, nil
}

func handleCheckoutSessionCompleted(ctx context.Context, event *stripeclient.Event) error {
	logger := loggercontext.Logger(ctx)
	var session stripeclient.CheckoutSession
	err := json.Unmarshal(event.Data.Raw, &session)
	if err != nil {
		logger.Errorw("Error parsing webhook JSON", "error", err)
		return fmt.Errorf("parsing webhook JSON: %w", err)
	}
	// Get the customer ID from the session
	customerId := session.Customer.ID
	logger.Infow("Session completed", "session_id", session.ID, "customer_id", customerId)
	return nil
}

func getSubscriptionCreated(ctx context.Context, event *stripeclient.Event) (*stripeclient.Subscription, error) {
	logger := loggercontext.Logger(ctx)
	var subscription stripeclient.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return nil, fmt.Errorf("parsing webhook JSON: %w", err)
	}
	logger.Infow("Subscription created", "subscription_id", subscription.ID)
	return &subscription, nil
}

func getSubscriptionDeleted(ctx context.Context, event *stripeclient.Event) (*stripeclient.Subscription, error) {
	logger := loggercontext.Logger(ctx)
	var subscription stripeclient.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return nil, fmt.Errorf("parsing customer.subscription.deleted webhook JSON: %w", err)
	}
	logger.Infow("Subscription deleted", "subscription_id", subscription.ID)
	return &subscription, nil
}

func getInvoicePaid(ctx context.Context, event *stripeclient.Event) (*stripeclient.Invoice, error) {
	logger := loggercontext.Logger(ctx)
	var invoice stripeclient.Invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		return nil, fmt.Errorf("parsing invoice.paid webhook JSON: %w", err)
	}
	logger.Infow("Invoice paid", "invoice_id", invoice.ID)
	return &invoice, nil
}

func getInvoiceFailed(ctx context.Context, event *stripeclient.Event) (*stripeclient.Invoice, error) {
	logger := loggercontext.Logger(ctx)
	var invoice stripeclient.Invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		return nil, fmt.Errorf("parsing invoice.failed webhook JSON: %w", err)
	}
	logger.Infow("Invoice failed", "invoice_id", invoice.ID)
	return &invoice, nil
}
