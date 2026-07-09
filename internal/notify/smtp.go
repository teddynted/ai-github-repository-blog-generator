package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
)

// SMTPClient sends mail through an authenticated SMTP relay such as Turbo SMTP
// (default host pro.turbo-smtp.com). It supports STARTTLS (port 587, the
// default) and implicit TLS (port 465); the connection is always encrypted and
// authenticated with PLAIN credentials.
type SMTPClient struct {
	Host     string // e.g. pro.turbo-smtp.com
	Port     int    // 587 (STARTTLS) or 465 (implicit TLS)
	Username string // Turbo SMTP account / API user
	Password string // Turbo SMTP password / API key
}

// NewSMTP builds an SMTPClient, defaulting the port to 587 when unset.
func NewSMTP(host string, port int, username, password string) *SMTPClient {
	if port == 0 {
		port = 587
	}
	return &SMTPClient{Host: host, Port: port, Username: username, Password: password}
}

// Send delivers msg to the recipients. The context bounds connection and I/O.
func (c *SMTPClient) Send(ctx context.Context, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}

	conn, err := c.dial(ctx, addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// STARTTLS upgrade for the plaintext port (465 is already wrapped in TLS).
	if c.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		} else {
			return fmt.Errorf("smtp: server %s does not offer STARTTLS on port %d", c.Host, c.Port)
		}
	}

	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}
	return client.Quit()
}

// dial opens the transport: implicit TLS for 465, plain TCP (upgraded later via
// STARTTLS) otherwise. Both honour the context for the connection phase.
func (c *SMTPClient) dial(ctx context.Context, addr string, tlsCfg *tls.Config) (net.Conn, error) {
	d := &net.Dialer{}
	if c.Port == 465 {
		return (&tls.Dialer{NetDialer: d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	}
	return d.DialContext(ctx, "tcp", addr)
}
