package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
)

// SMTPSender delivers a pre-built RFC 5322 message to its recipients over SMTP.
// *SMTPClient is the production implementation (Turbo SMTP); tests inject a fake.
type SMTPSender interface {
	Send(ctx context.Context, from string, to []string, msg []byte) error
}

// EmailNotifier sends run notifications by email over SMTP — configured for
// Turbo SMTP by default (see SMTPClient). The from address and recipients are
// whatever the Turbo SMTP account is allowed to send as.
type EmailNotifier struct {
	sender SMTPSender
	from   string
	to     []string
	logger *slog.Logger
}

// NewEmail builds an EmailNotifier over the given SMTP sender.
func NewEmail(sender SMTPSender, from string, to []string, logger *slog.Logger) *EmailNotifier {
	return &EmailNotifier{sender: sender, from: from, to: to, logger: logger}
}

// Notify sends one plain-text email describing the event.
func (n *EmailNotifier) Notify(ctx context.Context, e Event) error {
	subject := fmt.Sprintf("[blog-gen] %s: %s", e.Repo, e.Status)
	msg := buildMessage(n.from, n.to, subject, summarize(e))
	if err := n.sender.Send(ctx, n.from, n.to, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// buildMessage assembles a minimal, valid RFC 5322 plain-text message with CRLF
// line endings. The address of the sender is set verbatim; recipients are joined
// into the To header.
func buildMessage(from string, to []string, subject, body string) []byte {
	var b strings.Builder
	writeHeader(&b, "From", from)
	writeHeader(&b, "To", strings.Join(to, ", "))
	writeHeader(&b, "Subject", subject)
	writeHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", `text/plain; charset="UTF-8"`)
	b.WriteString("\r\n")
	// Normalise body line endings to CRLF.
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	return []byte(b.String())
}

func writeHeader(b *strings.Builder, key, value string) {
	// Strip CR/LF from header values to prevent header injection.
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\r\n")
}

// validAddresses reports whether every address parses, used by callers to
// fail fast on obvious misconfiguration.
func validAddresses(addrs ...string) bool {
	for _, a := range addrs {
		if _, err := mail.ParseAddress(a); err != nil {
			return false
		}
	}
	return true
}
