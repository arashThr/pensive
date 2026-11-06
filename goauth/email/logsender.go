package email

import "fmt"

// LogSender is a development email sender that prints messages to stdout
// instead of sending real emails. Use it in local development or tests.
type LogSender struct{}

// ForgotPassword prints the password-reset link to stdout.
func (l *LogSender) ForgotPassword(to, resetURL string) error {
	fmt.Printf("[email] ForgotPassword → %s | reset URL: %s\n", to, resetURL)
	return nil
}

// PasswordlessSignup prints the magic signup link to stdout.
func (l *LogSender) PasswordlessSignup(to, magicURL string) error {
	fmt.Printf("[email] PasswordlessSignup → %s | magic URL: %s\n", to, magicURL)
	return nil
}

// PasswordlessSignin prints the magic signin link to stdout.
func (l *LogSender) PasswordlessSignin(to, magicURL string) error {
	fmt.Printf("[email] PasswordlessSignin → %s | magic URL: %s\n", to, magicURL)
	return nil
}

// EmailVerification prints the verification link to stdout.
func (l *LogSender) EmailVerification(to, verificationURL string) error {
	fmt.Printf("[email] EmailVerification → %s | verify URL: %s\n", to, verificationURL)
	return nil
}
