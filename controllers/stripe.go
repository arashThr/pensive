package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/models"
	"github.com/stripe/stripe-go/v81"
	portalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/webhook"
)

type Stripe struct {
	Templates struct {
		Success Template
		Cancel  Template
	}
	Domain              string
	PriceId             string
	StripeWebhookSecret string
	StripeService       *models.StripeService
}

func (s Stripe) getStripeCustomerId(user *models.User) (customerId string, err error) {
	customerId, err = s.StripeService.GetCustomerIdByUserId(user.ID)
	if err == nil {
		return customerId, nil
	}
	if !errors.Is(err, models.ErrNoStripeCustomer) {
		return "", fmt.Errorf("get stripe customer id: %w", err)
	}
	log.Printf("No stripe customer found for user %v", user.ID)
	params := &stripe.CustomerListParams{Email: stripe.String(user.Email)}
	params.Filters.AddFilter("limit", "", "1")
	result := customer.List(params)
	if result.Next() {
		// Found customer
		log.Printf("Found stripe customer for user %v", user.ID)
		customer := result.Customer()
		customerId = customer.ID
	} else {
		// Create a new customer
		params := &stripe.CustomerParams{Email: stripe.String(user.Email)}
		params.AddMetadata("user_id", strconv.Itoa(int(user.ID)))
		// TODO: params.SetIdempotencyKey()
		customer, err := customer.New(params)
		if err != nil {
			if stripeErr, ok := err.(*stripe.Error); ok {
				log.Printf("Stripe error: %v", stripeErr.Error())
			} else {
				log.Printf("Create stripe customer error: %v", err)
			}
			return "", err
		}
		log.Printf("Created stripe customer for user %v with customer id %v", user.ID, customer.ID)
		customerId = customer.ID
	}
	s.StripeService.InsertCustomerId(user.ID, customerId)
	return customerId, nil
}

func (s Stripe) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	log.Printf("create checkout session for user %v", user.ID)
	customerId, err := s.getStripeCustomerId(user)
	if err != nil {
		log.Printf("getStripeCustomerId: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(s.PriceId),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.Domain + "/payments/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.Domain + "/payments/cancel"),
		Customer:   &customerId,
	}

	sess, err := session.New(checkoutParams)
	if err != nil {
		log.Printf("session.New: %v", err)
	}

	// TODO: Save session and customer ID in db
	// err = s.StripeService.SaveSession(user.ID, sess.ID)
	// if err != nil {
	// 	log.Printf("s.StripeService.SaveSession: %v", err)
	// 	http.Error(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }
	log.Printf("checkout session: %v\n", sess.ID)
	http.Redirect(w, r, sess.URL, http.StatusSeeOther)
}

func (s Stripe) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	log.Printf("create portal session for user %v", user.ID)
	// For demonstration purposes, we're using the Checkout session to retrieve the customer ID.
	// Typically this is stored alongside the authenticated user in your database.
	// TODO: Read session ID from db
	sessionId := r.FormValue("session_id")
	slog.Info("create portal session", "sessionId", sessionId)
	sess, err := session.Get(sessionId, nil)

	if err != nil {
		slog.Error("session.Get", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sess.Customer.ID),
		ReturnURL: stripe.String(s.Domain),
	}
	ps, err := portalsession.New(params)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Error("create portal session", "error", err)
		return
	}

	http.Redirect(w, r, ps.URL, http.StatusSeeOther)
}

func (s Stripe) GoToBillingPortal(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	customerId, err := s.StripeService.GetCustomerIdByUserId(user.ID)
	if err != nil {
		slog.Error("get stripe customer id", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	returnPath, err := url.JoinPath(s.Domain, "/users/me")
	if err != nil {
		slog.Error("url.JoinPath", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerId),
		ReturnURL: stripe.String(returnPath),
	}

	ps, err := portalsession.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Error("Go to billing portal", "error", err)
		return
	}

	http.Redirect(w, r, ps.URL, http.StatusSeeOther)
}

func (s Stripe) Success(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	log.Printf("user %v completed the checkout session", user.ID)
	// TODO: Should validate subscription here?
	var data struct{ SessionId string }
	data.SessionId = r.URL.Query().Get("session_id")
	s.Templates.Success.Execute(w, r, data)
}

func (s Stripe) Cancel(w http.ResponseWriter, r *http.Request) {
	// TODO: Keep track of payment statuses?
	s.Templates.Cancel.Execute(w, r, nil)
}

func (s Stripe) Webhook(w http.ResponseWriter, r *http.Request) {
	log.Printf("received webhook")

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
	// TODO: Which events I need?
	// TODO: Record in DB?
	switch event.Type {
	case "customer.subscription.deleted":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Subscription deleted for %v.", subscription.ID)
		// Then define and call a func to handle the deleted subscription.
		s.StripeService.HandleSubscriptionDeleted(&subscription)
		// handleSubscriptionCanceled(subscription)
	case "customer.subscription.updated":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Subscription updated for %v.", subscription.ID)
		// HERE: Pass prev attributes
		slog.Info("Subscription updated", "prevAtt", event.Data.PreviousAttributes)
		s.StripeService.HandleSubscriptionUpdated(&subscription)
	case "customer.subscription.created":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Subscription created for %v.", subscription.ID)
		// handleSubscriptionCreated(subscription)
	case "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Subscription trial will end for %v.", subscription.ID)
		// handleSubscriptionTrialWillEnd(subscription)
	case "entitlements.active_entitlement_summary.updated":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Active entitlement summary updated for %v.", subscription.ID)
		// Then define and call a func to handle active entitlement summary updated.
		// handleEntitlementUpdated(subscription)
	case "checkout.session.completed":
		// Payment is successful and the subscription is created.
		// You should provision the subscription and save the customer ID to your database.
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			return
		}
		log.Printf("Session completed: %s", session.ID)
		// Get the customer ID from the session
		customerId := session.Customer.ID
		userId, err := s.StripeService.GetUserIdByStripeCustomerId(customerId)
		if err != nil {
			log.Printf("Error getting user id for %v: %v", customerId, err)
			return
		}
		log.Printf("Processing session for user %d: %s", userId, session.ID)

	case "invoice.paid":
		// Continue to provision the subscription as payments continue to be made.
		// Store the status in your database and check when a user accesses your service.
		// This approach helps you avoid hitting rate limits.
		// Parse the event data to get the subscription details
		log.Printf("Invoice paid")
		var invoice stripe.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		go s.StripeService.HandleInvoicePaid(&invoice)
	case "invoice.payment_failed":
		// The payment failed or the customer does not have a valid payment method.
		// The subscription becomes past_due. Notify your customer and send them to the
		// customer portal to update their payment information.
		log.Printf("Invoice payment failed")
	default:
		fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
	}
	w.WriteHeader(http.StatusOK)
}

// SUCCESS
/*
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var event stripe.Event
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    switch event.Type {
    case "invoice.paid":
        // Parse the event data to get the subscription details
        invoice := event.Data.Object.(*stripe.Invoice)
        if invoice.Subscription != nil {
            // Update user to premium
            updateUserToPremium(*invoice.Subscription.ID)
        }
    }
}

func updateUserToPremium(subscriptionID string) {
    // Here you would implement the logic to update your user's status
    // Example: database update, API call to another service, etc.
}
*/

// FAILURE
/*
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var event stripe.Event
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    switch event.Type {
    case "invoice.payment_failed":
        invoice := event.Data.Object.(*stripe.Invoice)
        handlePaymentFailure(invoice)

    case "customer.subscription.deleted":
        subscription := event.Data.Object.(*stripe.Subscription)
        handleSubscriptionDeletion(subscription)
    }
}

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

func notifyUserOfPaymentIssue(subscriptionID string) {
    // Implement logic to notify user via email, in-app message, etc.
}

func downgradeUserFromPremium(customerID string) {
    // Implement logic to remove premium features or change user status
}
*/
