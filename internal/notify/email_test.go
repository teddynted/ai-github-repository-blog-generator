package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type fakeSES struct {
	in  *sesv2.SendEmailInput
	err error
}

func (f *fakeSES) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.in = in
	return &sesv2.SendEmailOutput{}, f.err
}

func TestEmailNotifierSends(t *testing.T) {
	f := &fakeSES{}
	n := NewEmail(f, "from@example.com", []string{"ops@example.com"}, nil)

	if err := n.Notify(context.Background(), Event{Repo: "acme/widget", Status: StatusPublished, Assets: 5}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if aws.ToString(f.in.FromEmailAddress) != "from@example.com" {
		t.Errorf("from = %q", aws.ToString(f.in.FromEmailAddress))
	}
	if len(f.in.Destination.ToAddresses) != 1 || f.in.Destination.ToAddresses[0] != "ops@example.com" {
		t.Errorf("to = %v", f.in.Destination.ToAddresses)
	}
	subject := aws.ToString(f.in.Content.Simple.Subject.Data)
	if subject == "" || aws.ToString(f.in.Content.Simple.Body.Text.Data) == "" {
		t.Errorf("subject/body empty: %q", subject)
	}
}

func TestEmailNotifierPropagatesError(t *testing.T) {
	n := NewEmail(&fakeSES{err: errors.New("throttled")}, "f@x", []string{"t@x"}, nil)
	if err := n.Notify(context.Background(), Event{Repo: "a/b", Status: StatusFailed}); err == nil {
		t.Error("expected send error to propagate")
	}
}
