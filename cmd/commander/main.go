package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Yusomii/agentic-nhi/pkg/bedrock"
	"github.com/Yusomii/agentic-nhi/pkg/slack"
	"github.com/aws/aws-lambda-go/events"
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

func HandleRequest(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSEventResponse, error) {
	var response events.SQSEventResponse

	for _, record := range sqsEvent.Records {
		var event EventBridgeEvent
		
		// 1. Unwrap the SQS Envelope
		if err := json.Unmarshal([]byte(record.Body), &event); err != nil {
			fmt.Printf("❌ SQS Unwrap Fail (Malformed): %v\n", err)
			continue // Do not return error, drop malformed payload so it doesn't loop
		}

		userName := event.Detail.RequestParameters.UserName
		keyId := event.Detail.ResponseElements.AccessKey.AccessKeyId

		if keyId == "" || userName == "" {
			fmt.Println("⚠️ Ignored: Missing Key ID or Username")
			continue
		}

		// 2. Initialize Bedrock Client
		b, err := bedrock.NewClient(ctx)
		if err != nil {
			fmt.Printf("❌ bedrock init failed: %v\n", err)
			response.BatchItemFailures = append(response.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			continue
		}

		// 3. Execute AI Telemetry Analysis using the raw CloudTrail string
		isRogue, err := b.AnalyzeCloudTrailEvent(ctx, json.RawMessage(record.Body))
		if err != nil {
			fmt.Printf("❌ bedrock analysis failed: %v\n", err)
			response.BatchItemFailures = append(response.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			continue
		}

		// 4. Conditional HITL Alerting
		if isRogue {
			s, err := slack.NewClient()
			if err != nil {
				fmt.Printf("❌ slack init failed: %v\n", err)
				response.BatchItemFailures = append(response.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
				continue
			}

			if err := s.SendKillSwitchAlert(ctx, userName, keyId, "Claude Sonnet identified anomalous IAM activity."); err != nil {
				fmt.Printf("❌ slack alert failed: %v\n", err)
				response.BatchItemFailures = append(response.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
				continue
			}
			fmt.Printf("✅ ALERT SENT: %s\n", keyId)
		} else {
			fmt.Printf("✅ EVENT VERIFIED BENIGN: %s\n", keyId)
		}
	}
	
	// Successful execution tells SQS to delete the message (except failed ones)
	return response, nil 
}

func main() { lambda.Start(HandleRequest) }

func init() { fmt.Println("V8_BINARY_FORCED_UPDATE_SQS") }