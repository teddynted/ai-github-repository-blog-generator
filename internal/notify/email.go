package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESAPI is the subset of the SES v2 client this notifier uses.
type SESAPI interface {
	SendEmail(ctx context.Context, in *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// EmailNotifier sends run notifications by email via Amazon SES. Both the from
// and to addresses must be verified/allowed in SES.
type EmailNotifier struct {
	api    SESAPI
	from   string
	to     []string
	logger *slog.Logger
}

// NewEmail builds an EmailNotifier.
func NewEmail(api SESAPI, from string, to []string, logger *slog.Logger) *EmailNotifier {
	return &EmailNotifier{api: api, from: from, to: to, logger: logger}
}

// Notify sends one email describing the event.
func (n *EmailNotifier) Notify(ctx context.Context, e Event) error {
	subject := fmt.Sprintf("[blog-gen] %s: %s", e.Repo, e.Status)
	_, err := n.api.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(n.from),
		Destination:      &sestypes.Destination{ToAddresses: n.to},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: aws.String(subject)},
				Body: &sestypes.Body{
					Text: &sestypes.Content{Data: aws.String(summarize(e))},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
