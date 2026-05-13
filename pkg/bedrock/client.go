package bedrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type Client struct {
	api *bedrockruntime.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	return &Client{
		api: bedrockruntime.NewFromConfig(cfg),
	}, nil
}

type claudeRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Messages         []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Client) AnalyzeCloudTrailEvent(ctx context.Context, eventJSON []byte) (bool, error) {
	systemPrompt := "You are a Zero-Trust Cloud Security Engine. Analyze this AWS CloudTrail event. Reply with ONLY a JSON object containing a boolean field 'is_rogue'. True if the action indicates a compromised Non-Human Identity, false otherwise. Return absolutely no text, markdown, or explanation other than the raw JSON object."

	payload := claudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        500,
		Messages: []message{
			{
				Role:    "user",
				Content: fmt.Sprintf("%s\n\nEvent:\n%s", systemPrompt, string(eventJSON)),
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal bedrock payload: %w", err)
	}

	modelID := "anthropic.claude-3-5-sonnet-20240620-v2:0"
	contentType := "application/json"

	input := &bedrockruntime.InvokeModelInput{
		ModelId:     &modelID,
		ContentType: &contentType,
		Body:        payloadBytes,
	}

	output, err := c.api.InvokeModel(ctx, input)
	if err != nil {
		return false, fmt.Errorf("failed to invoke claude 3.5 sonnet: %w", err)
	}

	if len(output.Body) == 0 {
		return false, fmt.Errorf("received empty response from bedrock")
	}

	var responseBody struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(output.Body, &responseBody); err != nil {
		return false, fmt.Errorf("failed to unmarshal bedrock response body: %w", err)
	}

	if len(responseBody.Content) > 0 {
		// Clean the string in case Claude wraps it in markdown
		rawText := responseBody.Content[0].Text
		if rawText == `{"is_rogue": true}` || rawText == `{"is_rogue":true}` {
			return true, nil
		}
	}

	return false, nil
}
