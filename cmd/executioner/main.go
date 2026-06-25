package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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

func verifySlackSignature(headers map[string]string, body string, secret string) bool {
	if secret == "" {
		fmt.Println("❌ SLACK_SIGNING_SECRET is not set")
		return false
	}
	var sig, timestamp string
	for k, v := range headers {
		lowerK := strings.ToLower(k)
		if lowerK == "x-slack-signature" {
			sig = v
		}
		if lowerK == "x-slack-request-timestamp" {
			timestamp = v
		}
	}
	if sig == "" || timestamp == "" {
		return false
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 300 {
		return false // replay attack
	}

	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	mySig := "v0=" + hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(mySig), []byte(sig))
}

func HandleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	body := request.Body
	if request.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err == nil {
			body = string(decoded)
		}
	}

	secret := os.Getenv("SLACK_SIGNING_SECRET")
	if !verifySlackSignature(request.Headers, body, secret) {
		fmt.Println("❌ INVALID SLACK SIGNATURE")
		return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: "Unauthorized"}, nil
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