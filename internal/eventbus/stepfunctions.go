package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
)

// SFNAPI is the subset of the Step Functions client this adapter uses.
type SFNAPI interface {
	StartExecution(ctx context.Context, in *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
}

// StepFunctionsPublisher starts one execution of the orchestration state
// machine per intake event. The machine starts the compute host, waits for it
// to report ready via SSM, and enqueues the job onto SQS for the worker to
// drain. It satisfies intake.Publisher.
type StepFunctionsPublisher struct {
	api             SFNAPI
	stateMachineArn string
}

// NewStepFunctions builds a publisher targeting the given state machine ARN.
func NewStepFunctions(api SFNAPI, stateMachineArn string) *StepFunctionsPublisher {
	return &StepFunctionsPublisher{api: api, stateMachineArn: stateMachineArn}
}

// Publish starts a state-machine execution whose input is exactly the SQS
// message body the worker consumes — {"detail": <event>} — so the machine can
// forward it to the events queue verbatim once the host reports ready.
func (p *StepFunctionsPublisher) Publish(ctx context.Context, ev intake.Event) error {
	input, err := json.Marshal(map[string]any{"detail": ev})
	if err != nil {
		return fmt.Errorf("marshal execution input: %w", err)
	}
	if _, err := p.api.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(p.stateMachineArn),
		Input:           aws.String(string(input)),
	}); err != nil {
		return fmt.Errorf("start execution: %w", err)
	}
	return nil
}
