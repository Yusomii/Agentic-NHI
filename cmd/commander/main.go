package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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

func HandleRequest(ctx context.Context, event EventBridgeEvent) (string, error) {
	userName := event.Detail.RequestParameters.UserName
	keyId := event.Detail.ResponseElements.AccessKey.AccessKeyId

	if keyId == "" {
		return "Ignored: No Key ID", nil
	}

	payload := map[string]interface{}{
		"channel": os.Getenv("SLACK_CHANNEL_ID"),
		"blocks": []interface{}{
			map[string]interface{}{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*🚨 AGENTIC-NHI: Rogue Access Key Detected*\n*User:* %s\n*Key:* `%s`", userName, keyId),
				},
			},
			map[string]interface{}{
				"type": "actions",
				"elements": []interface{}{
					map[string]interface{}{
						"type":      "button",
						"text":      map[string]string{"type": "plain_text", "text": "Approve (Deactivate Key)"},
						"style":     "danger",
						"action_id": "deactivate_key",
						"value":     fmt.Sprintf("%s|%s", userName, keyId),
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SLACK_BOT_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack call failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("✅ ALERT SENT: %s\n", keyId)
	return "Alert Success", nil
}

func main() { lambda.Start(HandleRequest) }
