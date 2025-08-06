package service

import (
	"fmt"

	"github.com/arashthr/go-course/internal/config"
	"gopkg.in/mail.v2"
)

const (
	DefaultSender = "me@noreply.com"
)

type EmailService struct {
	DefaultSender string
	dialer        *mail.Dialer
}

type Email struct {
	From      string
	To        string
	Subject   string
	Plaintext string
	HTML      string
}

func NewEmailService(config config.SMTPConfig) *EmailService {
	return &EmailService{
		dialer: mail.NewDialer(
			config.Host, config.Port, config.Username, config.Password,
		),
	}
}

func (es *EmailService) Send(email Email) error {
	msg := mail.NewMessage()
	es.setFrom(msg, email)
	msg.SetHeader("To", email.To)
	msg.SetHeader("Subject", email.Subject)
	switch {
	case email.Plaintext != "" && email.HTML != "":
		msg.SetBody("text/plain", email.Plaintext)
		msg.AddAlternative("text/html", email.HTML)
	case email.Plaintext != "":
		msg.SetBody("text/plain", email.Plaintext)
	case email.HTML != "":
		msg.SetBody("text/html", email.HTML)
	default:
		return fmt.Errorf("email must have either plaintext or HTML content")
	}
	err := es.dialer.DialAndSend(msg)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func (es *EmailService) ForgotPassword(to, resetURL string) error {
	email := Email{
		Subject:   "Reset your password",
		To:        to,
		Plaintext: "To reset your password, please visit the following link: " + resetURL,
		HTML:      `<p>To reset your password, please visit the following link: <a href="` + resetURL + `">` + resetURL + `</a></p>`,
	}
	err := es.Send(email)
	if err != nil {
		return fmt.Errorf("forgot password email: %w", err)
	}
	return nil
}

func (es *EmailService) PasswordlessSignup(to, magicURL string) error {
	plaintext := fmt.Sprintf(`Welcome to Pensive!

Click the link below to complete your sign up:
%s

This link will expire in 15 minutes for security reasons.

If you didn't request this, please ignore this email.

---
The Pensive Team`, magicURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Complete your sign up</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="text-align: center; margin-bottom: 30px;">
        <h1 style="color: #000; margin-bottom: 10px;">Welcome to Pensive!</h1>
        <p style="color: #666; font-size: 16px;">Click the button below to complete your sign up</p>
    </div>
    
    <div style="text-align: center; margin: 30px 0;">
        <a href="%s" style="background-color: #000; color: #fff; padding: 12px 30px; text-decoration: none; font-weight: bold; border-radius: 4px; display: inline-block;">Complete Sign Up</a>
    </div>
    
    <div style="border-top: 2px solid #eee; padding-top: 20px; margin-top: 30px;">
        <p style="color: #666; font-size: 14px; margin-bottom: 10px;">
            <strong>This link will expire in 15 minutes</strong> for security reasons.
        </p>
        <p style="color: #666; font-size: 14px;">
            If you didn't request this, please ignore this email.
        </p>
        <p style="color: #666; font-size: 14px; margin-top: 20px;">
            —<br>The Pensive Team
        </p>
    </div>
</body>
</html>`, magicURL)

	email := Email{
		Subject:   "Complete your sign up",
		To:        to,
		Plaintext: plaintext,
		HTML:      html,
	}
	err := es.Send(email)
	if err != nil {
		return fmt.Errorf("passwordless signup email: %w", err)
	}
	return nil
}

func (es *EmailService) PasswordlessSignin(to, magicURL string) error {
	plaintext := fmt.Sprintf(`Sign in to Pensive

Click the link below to sign in to your account:
%s

This link will expire in 15 minutes for security reasons.

If you didn't request this, please ignore this email.

---
The Pensive Team`, magicURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign in to your account</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="text-align: center; margin-bottom: 30px;">
        <h1 style="color: #000; margin-bottom: 10px;">Sign in to Pensive</h1>
        <p style="color: #666; font-size: 16px;">Click the button below to access your account</p>
    </div>
    
    <div style="text-align: center; margin: 30px 0;">
        <a href="%s" style="background-color: #000; color: #fff; padding: 12px 30px; text-decoration: none; font-weight: bold; border-radius: 4px; display: inline-block;">Sign In</a>
    </div>
    
    <div style="border-top: 2px solid #eee; padding-top: 20px; margin-top: 30px;">
        <p style="color: #666; font-size: 14px; margin-bottom: 10px;">
            <strong>This link will expire in 15 minutes</strong> for security reasons.
        </p>
        <p style="color: #666; font-size: 14px;">
            If you didn't request this, please ignore this email.
        </p>
        <p style="color: #666; font-size: 14px; margin-top: 20px;">
            —<br>The Pensive Team
        </p>
    </div>
</body>
</html>`, magicURL)

	email := Email{
		Subject:   "Sign in to your account",
		To:        to,
		Plaintext: plaintext,
		HTML:      html,
	}
	err := es.Send(email)
	if err != nil {
		return fmt.Errorf("passwordless signin email: %w", err)
	}
	return nil
}

func (es *EmailService) setFrom(msg *mail.Message, email Email) {
	var from string
	switch {
	case email.From != "":
		from = email.From
	case es.DefaultSender != "":
		from = es.DefaultSender
	default:
		from = DefaultSender
	}
	msg.SetHeader("From", from)
}
