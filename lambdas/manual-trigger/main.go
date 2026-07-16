// Command manual-trigger is the AWS Lambda behind the authenticated
// POST /process REST endpoint. It is one more trigger source for the platform:
// it validates a JSON request, then hands it to the shared intake.Service,
// which publishes onto the same EventBridge → SQS → worker pipeline the webhook
// uses. It performs no cloning, analysis, or AI work, and never starts the EC2
// instance — outside the operating window the request is rejected.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/eventbus"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
)

// triggerSource labels events and logs produced by this entry point.
const triggerSource = "manual"

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	// ProjectName is required here (unlike the webhook): the manual trigger must
	// know whether the platform is inside its operating window so it can reject
	// out-of-window requests rather than start the instance.
	if err := a.Config.Require("AWSRegion", "EventBusName", "EventSource", "ProjectName"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	svc := &intake.Service{
		Publisher: eventbus.NewEventBridge(eventbridge.NewFromConfig(awsCfg), a.Config.EventBusName, a.Config.EventSource),
		Window:    awsec2.NewInstanceWindow(awsec2.New(ec2.NewFromConfig(awsCfg)), a.Config.ProjectName),
		Logger:    a.Logger,
	}

	lambda.Start(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		body := []byte(req.Body)
		if req.IsBase64Encoded {
			if decoded, derr := base64.StdEncoding.DecodeString(req.Body); derr == nil {
				body = decoded
			}
		}
		status, respBody := handle(ctx, svc, a.Logger, req.RequestContext.RequestID, body)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(respBody),
		}, nil
	})
}

// acceptedResponse is the success body: HTTP 202.
type acceptedResponse struct {
	Status    string `json:"status"`
	Trigger   string `json:"trigger"`
	RequestID string `json:"requestId"`
	Message   string `json:"message"`
}

// errorResponse is the rejection/validation/error body.
type errorResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// handle validates the request, submits it through the shared intake service,
// and maps the outcome to an HTTP status + JSON body. It is separated from the
// Lambda wiring so it is unit-testable. It never starts the instance.
func handle(ctx context.Context, svc *intake.Service, logger *slog.Logger, requestID string, body []byte) (int, []byte) {
	start := time.Now()

	var r intake.Request
	if err := json.Unmarshal(body, &r); err != nil {
		return jsonErr(400, "invalid JSON body")
	}
	if err := r.Validate(); err != nil {
		return jsonErr(400, err.Error())
	}

	ev := r.ToEvent(triggerSource)
	decision, err := svc.Submit(ctx, ev, intake.RejectOutsideWindow)
	logResult(logger, requestID, ev, decision, err, time.Since(start))

	switch {
	case err != nil:
		return jsonErr(500, "internal error")
	case decision == intake.Rejected:
		// Outside the operating window: nothing published, instance not started.
		return jsonResp(503, errorResponse{
			Status: "rejected",
			Reason: "AI platform is currently outside operational hours.",
		})
	default: // Accepted
		return jsonResp(202, acceptedResponse{
			Status:    "accepted",
			Trigger:   triggerSource,
			RequestID: requestID,
			Message:   "AI processing has been initiated.",
		})
	}
}

// logResult emits one structured line identifying the trigger source and run
// context. It logs no secrets or request bodies.
func logResult(logger *slog.Logger, requestID string, ev intake.Event, decision intake.Decision, err error, dur time.Duration) {
	if logger == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	logger.Info("manual trigger processed",
		slog.String("trigger_source", triggerSource),
		slog.String("request_id", requestID),
		slog.String("repository", ev.RepoFullName),
		slog.String("branch", ev.Ref),
		slog.String("provider", ev.Provider),
		slog.String("decision", string(decision)),
		slog.Duration("duration", dur),
		slog.String("outcome", outcome))
}

func jsonResp(status int, v any) (int, []byte) {
	b, _ := json.Marshal(v)
	return status, b
}

func jsonErr(status int, reason string) (int, []byte) {
	return jsonResp(status, errorResponse{Status: "error", Reason: reason})
}
