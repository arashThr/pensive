package email

import (
	"fmt"

	"gopkg.in/mail.v2"
)

// Config holds SMTP connection parameters.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	// From is the default sender address.
	From string
}

// Email represents a single message to be sent.
type Email struct {
	From      string
	To        string
	Subject   string
	Plaintext string
	HTML      string
}

// Service sends transactional emails via SMTP.
type Service struct {
	from   string
	dialer *mail.Dialer
}

// NewService creates a new email Service from the given config.
func NewService(cfg Config) *Service {
	return &Service{
		from:   cfg.From,
		dialer: mail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password),
	}
}

// Send dispatches a single email.
func (s *Service) Send(e Email) error {
	msg := mail.NewMessage()
	from := e.From
	if from == "" {
		from = s.from
	}
	msg.SetHeader("From", from)
	msg.SetHeader("To", e.To)
	msg.SetHeader("Subject", e.Subject)

	switch {
	case e.Plaintext != "" && e.HTML != "":
		msg.SetBody("text/plain", e.Plaintext)
		msg.AddAlternative("text/html", e.HTML)
	case e.HTML != "":
		msg.SetBody("text/html", e.HTML)
	case e.Plaintext != "":
		msg.SetBody("text/plain", e.Plaintext)
	default:
		return fmt.Errorf("email: must have plaintext or HTML body")
	}

	if err := s.dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("email send: %w", err)
	}
	return nil
}

// ForgotPassword sends a password-reset link.
func (s *Service) ForgotPassword(to, resetURL string) error {
	return s.Send(Email{
		To:        to,
		Subject:   "Reset your password",
		Plaintext: "To reset your password visit: " + resetURL,
		HTML:      fmt.Sprintf(`<p>To reset your password click: <a href="%s">%s</a></p>`, resetURL, resetURL),
	})
}

// PasswordlessSignup sends a magic-link for account creation.
func (s *Service) PasswordlessSignup(to, magicURL string) error {
	return s.Send(Email{
		To:      to,
		Subject: "Complete your sign up",
		Plaintext: fmt.Sprintf(
			"Welcome! Complete your sign up by visiting this link (expires in 15 minutes):\n%s", magicURL),
		HTML: fmt.Sprintf(`<p>Welcome! <a href="%s">Complete your sign up</a> (expires in 15 minutes).</p>`, magicURL),
	})
}

// PasswordlessSignin sends a magic-link for sign-in.
func (s *Service) PasswordlessSignin(to, magicURL string) error {
	return s.Send(Email{
		To:      to,
		Subject: "Sign in to your account",
		Plaintext: fmt.Sprintf(
			"Click this link to sign in (expires in 15 minutes):\n%s", magicURL),
		HTML: fmt.Sprintf(`<p><a href="%s">Sign in</a> (expires in 15 minutes).</p>`, magicURL),
	})
}

// EmailVerification sends an email-address verification link.
func (s *Service) EmailVerification(to, verificationURL string) error {
	return s.Send(Email{
		To:      to,
		Subject: "Verify your email address",
		Plaintext: fmt.Sprintf(
			"Verify your email by visiting (expires in 15 minutes):\n%s", verificationURL),
		HTML: fmt.Sprintf(`<p><a href="%s">Verify your email</a> (expires in 15 minutes).</p>`, verificationURL),
	})
}
