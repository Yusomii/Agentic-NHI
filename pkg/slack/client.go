package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/slack-go/slack"
)

type Client struct {
	api       *slack.Client
	channelID string
}

func NewClient() (*Client, error) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is missing")
	}
	channelID := os.Getenv("SLACK_CHANNEL_ID")
	if channelID == "" {
		return nil, fmt.Errorf("SLACK_CHANNEL_ID is missing")
	}
	return &Client{
		api:       slack.New(token),
		channelID: channelID,
	}, nil
}

// SendKillSwitchAlert constructs an interactive Block Kit UI and sends it to Slack
func (c *Client) SendKillSwitchAlert(ctx context.Context, principal string, accessKey string, reason string) error {
	// 1. The hidden payload embedded in the button for API Gateway
	type btnPayload struct {
		Action string `json:"action"`
		User   string `json:"user"`
		Key    string `json:"key"`
	}
	payloadBytes, _ := json.Marshal(btnPayload{Action: "deactivate", User: principal, Key: accessKey})
	targetPayload := string(payloadBytes)

	// 2. Build the Header/Reasoning Block
	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🚨 *High-Risk IAM Anomaly Detected*\n*Principal:* `%s`\n*Target Key:* `%s`\n*AI Assessment:* %s", principal, accessKey, reason), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	// 3. Build the Interactive Buttons
	approveBtnText := slack.NewTextBlockObject("plain_text", "Approve (Deactivate Key)", false, false)
	// Notice we inject 'targetPayload' into the button's Value parameter here
	approveBtn := slack.NewButtonBlockElement("execute_kill_switch", targetPayload, approveBtnText)
	approveBtn.Style = slack.StyleDanger

	denyBtnText := slack.NewTextBlockObject("plain_text", "Deny (False Positive)", false, false)
	denyBtn := slack.NewButtonBlockElement("ignore_alert", "ignore", denyBtnText)

	actionBlock := slack.NewActionBlock("actions", approveBtn, denyBtn)

	// 4. Transmit to Slack
	_, _, err := c.api.PostMessageContext(ctx, c.channelID, slack.MsgOptionBlocks(headerSection, actionBlock))
	if err != nil {
		return fmt.Errorf("failed to send interactive slack alert: %w", err)
	}

	return nil
}

// Fallback method for basic errors (keeps your Graceful Degradation working)
func (c *Client) SendAlert(ctx context.Context, eventSource, eventName string, isRogue bool) error {
	if !isRogue {
		return nil
	}
	header := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*🚨 Rogue NHI Activity Detected*", false, false), nil, nil)
	details := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("System flagged:\n*Source:* %s\n*Action:* %s", eventSource, eventName), false, false), nil, nil)

	_, _, err := c.api.PostMessageContext(ctx, c.channelID, slack.MsgOptionBlocks(header, details))
	return err
}
