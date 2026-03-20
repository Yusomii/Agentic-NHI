package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type SlackInteraction struct {
	Type    string `json:"type"`
	Actions []struct {
		ActionID string `json:"action_id"`
		Value    string `json:"value"`
	} `json:"actions"`
}

type ActionPayload struct {
	Action string `json:"action"`
	User   string `json:"user"`
	Key    string `json:"key"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	slog.Info("received webhook invocation from API Gateway")

	parsedBody, err := url.ParseQuery(req.Body)
	if err != nil {
		slog.Error("failed to parse url-encoded body", slog.String("error", err.Error()))
		return clientError(http.StatusBadRequest, "invalid encoding")
	}

	payloadStr := parsedBody.Get("payload")
	if payloadStr == "" {
		slog.Error("missing payload key in request")
		return clientError(http.StatusBadRequest, "missing payload")
	}

	var interaction SlackInteraction
	if err := json.Unmarshal([]byte(payloadStr), &interaction); err != nil {
		slog.Error("failed to unmarshal slack interaction", slog.String("error", err.Error()))
		return clientError(http.StatusBadRequest, "invalid json envelope")
	}

	if len(interaction.Actions) == 0 {
		return clientError(http.StatusBadRequest, "no actions found")
	}
	action := interaction.Actions[0]

	if action.ActionID == "ignore_alert" {
		slog.Info("human operator marked alert as false positive")
		return successResponse()
	}

	if action.ActionID == "execute_kill_switch" {
		var target ActionPayload
		if err := json.Unmarshal([]byte(action.Value), &target); err != nil {
			slog.Error("failed to parse hidden button value", slog.String("error", err.Error()))
			return clientError(http.StatusInternalServerError, "invalid internal payload")
		}

		slog.Warn("EXECUTING KILL SWITCH", slog.String("target_key", target.Key), slog.String("target_user", target.User))
		
		// THE EXECUTION
		if err := deactivateAccessKey(ctx, target.User, target.Key); err != nil {
			slog.Error("failed to deactivate access key", slog.String("error", err.Error()))
			return clientError(http.StatusInternalServerError, "failed to execute AWS IAM command")
		}

		slog.Info("THREAT NEUTRALIZED. IAM KEY IS NOW INACTIVE.")
		return successResponse()
	}

	return successResponse()
}

// deactivateAccessKey uses the AWS SDK to toggle the rogue key status to Inactive
func deactivateAccessKey(ctx context.Context, userName string, accessKeyID string) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %w", err)
	}

	client := iam.NewFromConfig(cfg)

	_, err = client.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
		AccessKeyId: aws.String(accessKeyID),
		Status:      types.StatusTypeInactive,
		UserName:    aws.String(userName),
	})

	if err != nil {
		return fmt.Errorf("aws sdk error: %w", err)
	}

	return nil
}

func successResponse() (events.APIGatewayProxyResponse, error) {

	return events.APIGatewayProxyResponse{

		StatusCode: http.StatusOK, 

		Body: "OK",

	}, nil

}



func clientError(status int, msg string) (events.APIGatewayProxyResponse, error) {

	return events.APIGatewayProxyResponse{

		StatusCode: status, 

		Body: fmt.Sprintf(`{"error":"%s"}`, msg),

	}, nil

}
