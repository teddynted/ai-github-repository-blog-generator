// Command manual-trigger is the AWS Lambda behind the authenticated
// POST /process REST endpoint — the platform's Step Functions trigger. It
// validates a JSON request, then hands it to the shared intake.Service, which
// starts an execution of the orchestration state machine. The state machine
// starts the compute host, waits for it to report ready via SSM, and enqueues
// the job onto SQS for the worker. This Lambda performs no cloning, analysis,
// or AI work, and does not start the instance itself.
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
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
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
	// The state machine (not this Lambda) owns the compute-host lifecycle, so
	// only the region and the target state-machine ARN are required.
	if err := a.Config.Require("AWSRegion", "StateMachineArn"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	svc := &intake.Service{
		Publisher: eventbus.NewStepFunctions(sfn.NewFromConfig(awsCfg), a.Config.StateMachineArn),
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
	// Start the orchestration state machine; it starts the host, waits for it
	// to report ready via SSM, and enqueues the job for the worker.
	decision, err := svc.Submit(ctx, ev, intake.BufferOutsideWindow)
	logResult(logger, requestID, ev, decision, err, time.Since(start))

	if err != nil {
		return jsonErr(500, "internal error")
	}
	msg := "AI processing has been initiated."
	if decision == intake.Started {
		msg = "AI platform is starting; processing will begin shortly."
	} else if decision == intake.Deferred {
		msg = "AI platform could not be started now; processing will begin at the next scheduled runtime."
	}
	return jsonResp(202, acceptedResponse{
		Status:    "accepted",
		Trigger:   triggerSource,
		RequestID: requestID,
		Message:   msg,
	})
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
