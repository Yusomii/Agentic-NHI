package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/Yusomii/agentic-nhi/pkg/bedrock"
	"github.com/Yusomii/agentic-nhi/pkg/slack"
	"github.com/aws/aws-lambda-go/lambda"
)

type CloudTrailEvent struct {
	Records []CloudTrailRecord `json:"Records"`
}

type CloudTrailRecord struct {
	EventVersion      string          `json:"eventVersion"`
	EventTime         string          `json:"eventTime"`
	EventSource       string          `json:"eventSource"`
	EventName         string          `json:"eventName"`
	AwsRegion         string          `json:"awsRegion"`
	SourceIPAddress   string          `json:"sourceIPAddress"`
	UserIdentity      UserIdentity    `json:"userIdentity"`
	RequestParameters json.RawMessage `json:"requestParameters"`
	ResponseElements  json.RawMessage `json:"responseElements"`
}

type UserIdentity struct {
	Type        string `json:"type"`
	PrincipalID string `json:"principalId"`
	Arn         string `json:"arn"`
	AccountID   string `json:"accountId"`
	AccessKeyID string `json:"accessKeyId"`
	UserName    string `json:"userName"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, event CloudTrailEvent) error {
	slog.Info("received cloudtrail event", slog.Int("record_count", len(event.Records)))

	// 1. Initialize Adapters
	aiClient, err := bedrock.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize bedrock client: %w", err)
	}

	slackClient, err := slack.NewClient()
	if err != nil {
		return fmt.Errorf("failed to initialize slack client: %w", err)
	}

	// 2. Process Events
	for _, record := range event.Records {
		recordJSON, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal cloudtrail record: %w", err)
		}

		isRogue, err := aiClient.AnalyzeCloudTrailEvent(ctx, recordJSON)
		if err != nil {
			return fmt.Errorf("failed to analyze cloudtrail event: %w", err)
		}

		// 3. Human-in-the-Loop Routing
		if isRogue {
			slog.Warn("rogue behavior detected, requesting human approval")
			if err := slackClient.SendAlert(ctx, record.EventSource, record.EventName, isRogue); err != nil {
				return fmt.Errorf("failed to send slack alert: %w", err)
			}
		} else {
			slog.Info("event benign", slog.String("action", record.EventName))
		}
	}

	return nil
}