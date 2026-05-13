package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

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

type ActionValue struct {
	User string `json:"user"`
	Key  string `json:"key"`
}

func HandleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	body := request.Body
	if request.IsBase64Encoded {
		decoded, _ := base64.StdEncoding.DecodeString(body)
		body = string(decoded)
	}

	vals, _ := url.ParseQuery(body)
	payloadRaw := vals.Get("payload")
	if payloadRaw == "" {
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "OK"}, nil
	}

	var p SlackPayload
	if err := json.Unmarshal([]byte(payloadRaw), &p); err != nil {
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "JSON Error"}, nil
	}

	if len(p.Actions) == 0 {
		return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "No Actions"}, nil
	}

	// Aligned with Commander Payload Schema
	if p.Actions[0].ActionID == "execute_kill_switch" {
		var val ActionValue
		if err := json.Unmarshal([]byte(p.Actions[0].Value), &val); err == nil {
			_, err := iamSvc.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
				UserName:    &val.User,
				AccessKeyId: &val.Key,
				Status:      types.StatusTypeInactive,
			})
			if err != nil {
				fmt.Printf("❌ IAM FAIL: %v\n", err)
			} else {
				fmt.Printf("✅ KILLED: %s\n", val.Key)
			}
		} else {
			fmt.Printf("❌ VALUE PARSE FAIL: %v\n", err)
		}
	} else {
		fmt.Printf("❌ WRONG ACTION ID: %s\n", p.Actions[0].ActionID)
	}

	return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: "Processed"}, nil
}

func main() { lambda.Start(HandleRequest) }