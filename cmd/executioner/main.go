package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

var iamSvc *iam.Client

func init() {
	cfg, _ := config.LoadDefaultConfig(context.TODO())
	iamSvc = iam.NewFromConfig(cfg)
}

type SlackPayload struct {
	Actions []struct {
		ActionID string `json:"action_id"`
		Value    string `json:"value"`
	} `json:"actions"`
}

func HandleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	body := request.Body
	if request.IsBase64Encoded {
		decoded, _ := base64.StdEncoding.DecodeString(body)
		body = string(decoded)
	}

	fmt.Printf("DEBUG [PHASE 1]: Raw Body Length: %d\n", len(body))

	vals, _ := url.ParseQuery(body)
	payloadRaw := vals.Get("payload")
	if payloadRaw == "" {
		fmt.Println("DEBUG [SILENT DROP A]: payloadRaw is empty. Could not extract URL-encoded payload.")
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "OK"}, nil
	}

	var p SlackPayload
	if err := json.Unmarshal([]byte(payloadRaw), &p); err != nil {
		fmt.Printf("DEBUG [SILENT DROP B]: JSON Unmarshal error: %v\n", err)
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "JSON Error"}, nil
	}

	if len(p.Actions) == 0 {
		fmt.Println("DEBUG [SILENT DROP C]: No actions found in Slack payload array.")
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "No Actions"}, nil
	}

	fmt.Printf("DEBUG [PHASE 2]: Received ActionID: '%s' | Value: '%s'\n", p.Actions[0].ActionID, p.Actions[0].Value)

	if p.Actions[0].ActionID == "deactivate_key" {
		parts := strings.Split(p.Actions[0].Value, "|")
		if len(parts) == 2 {
			user, key := parts[0], parts[1]
			_, err := iamSvc.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
				UserName:    &user,
				AccessKeyId: &key,
				Status:      types.StatusTypeInactive,
			})
			if err != nil {
				fmt.Printf("❌ IAM FAIL: %v\n", err)
			} else {
				fmt.Printf("✅ KILLED: %s\n", key)
			}
		} else {
			fmt.Printf("DEBUG [SILENT DROP D]: Value splitting failed. Expected 2 parts, got %d.\n", len(parts))
		}
	} else {
		fmt.Printf("DEBUG [SILENT DROP E]: ActionID '%s' did not match expected 'deactivate_key'.\n", p.Actions[0].ActionID)
	}

	return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "Processed"}, nil
}

func main() { lambda.Start(HandleRequest) }