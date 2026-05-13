package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Yusomii/agentic-nhi/pkg/bedrock"
	"github.com/Yusomii/agentic-nhi/pkg/slack"
	"github.com/aws/aws-lambda-go/lambda"
)

type EventBridgeEvent struct {
	Detail struct {
		RequestParameters struct {
			UserName string `json:"userName"`
		} `json:"requestParameters"`
		ResponseElements struct {
			AccessKey struct {
				AccessKeyId string `json:"accessKeyId"`
			} `json:"accessKey"`
		} `json:"responseElements"`
	} `json:"detail"`
}

func HandleRequest(ctx context.Context, rawEvent json.RawMessage) (string, error) {
	var event EventBridgeEvent
	if err := json.Unmarshal(rawEvent, &event); err != nil {
		return "Ignored: Malformed Payload", nil
	}

	userName := event.Detail.RequestParameters.UserName
	keyId := event.Detail.ResponseElements.AccessKey.AccessKeyId

	if keyId == "" || userName == "" {
		return "Ignored: Missing Key ID or Username", nil
	}

	// 1. Initialize Bedrock Client
	b, err := bedrock.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to initialize bedrock client: %w", err)
	}

	// 2. Execute AI Telemetry Analysis
	isRogue, err := b.AnalyzeCloudTrailEvent(ctx, rawEvent)
	if err != nil {
		return "", fmt.Errorf("bedrock analysis failed: %w", err)
	}

	// 3. Conditional HITL Alerting
	if isRogue {
		s, err := slack.NewClient()
		if err != nil {
			return "", fmt.Errorf("failed to initialize slack client: %w", err)
		}

		if err := s.SendKillSwitchAlert(ctx, userName, keyId, "Claude 4.6 Sonnet identified anomalous IAM activity."); err != nil {
			return "", fmt.Errorf("failed to send slack alert: %w", err)
		}
		fmt.Printf("✅ ALERT SENT: %s\n", keyId)
		return "Alert Success", nil
	}

	fmt.Printf("✅ EVENT VERIFIED BENIGN: %s\n", keyId)
	return "Benign Event Ignored", nil
}

func main() { lambda.Start(HandleRequest) }

// FORCED_STATE_UPDATE_01
func init() { fmt.Println("V5_BINARY_FORCED_UPDATE") }
