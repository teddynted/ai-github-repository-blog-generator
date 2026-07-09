package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeSMTP struct {
	from string
	to   []string
	msg  []byte
	err  error
}

func (f *fakeSMTP) Send(_ context.Context, from string, to []string, msg []byte) error {
	f.from, f.to, f.msg = from, to, msg
	return f.err
}

func TestEmailNotifierSends(t *testing.T) {
	f := &fakeSMTP{}
	n := NewEmail(f, "from@example.com", []string{"ops@example.com", "eng@example.com"}, nil)

	if err := n.Notify(context.Background(), Event{Repo: "acme/widget", Status: StatusPublished, Assets: 5}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if f.from != "from@example.com" {
		t.Errorf("from = %q", f.from)
	}
	if len(f.to) != 2 || f.to[0] != "ops@example.com" {
		t.Errorf("to = %v", f.to)
	}

	msg := string(f.msg)
	for _, want := range []string{
		"From: from@example.com\r\n",
		"To: ops@example.com, eng@example.com\r\n",
		"Subject: [blog-gen] acme/widget: published\r\n",
		`Content-Type: text/plain; charset="UTF-8"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q\n---\n%s", want, msg)
		}
	}
	// Header block must be terminated by a blank CRLF line before the body.
	if !strings.Contains(msg, "\r\n\r\n") {
		t.Error("message has no header/body separator")
	}
}

func TestEmailNotifierPropagatesError(t *testing.T) {
	n := NewEmail(&fakeSMTP{err: errors.New("relay refused")}, "f@x.io", []string{"t@x.io"}, nil)
	if err := n.Notify(context.Background(), Event{Repo: "a/b", Status: StatusFailed}); err == nil {
		t.Error("expected send error to propagate")
	}
}

func TestBuildMessageStripsHeaderInjection(t *testing.T) {
	// A subject carrying CRLF must not be able to start a new header line.
	msg := string(buildMessage("f@x.io", []string{"t@x.io"}, "hi\r\nBcc: evil@x.io", "body"))
	if strings.Contains(msg, "\r\nBcc:") {
		t.Errorf("header injection not neutralised (Bcc began a new line):\n%s", msg)
	}
	// The folded value should survive as part of the Subject line.
	if !strings.Contains(msg, "Subject: hi  Bcc: evil@x.io\r\n") {
		t.Errorf("expected CRLF to be folded to spaces within Subject:\n%s", msg)
	}
}

func TestValidAddresses(t *testing.T) {
	if !validAddresses("a@b.io", "c@d.io") {
		t.Error("expected valid addresses to pass")
	}
	if validAddresses("not-an-address") {
		t.Error("expected invalid address to fail")
	}
}
