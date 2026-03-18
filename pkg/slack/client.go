package slack

import (
	"context"
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

func (c *Client) SendAlert(ctx context.Context, eventSource, eventName string, isRogue bool) error {
	if !isRogue {
		return nil
	}
	header := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*🚨 Rogue NHI Activity Detected*", false, false), nil, nil)
	details := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Claude flagged:\n*Source:* %s\n*Action:* %s", eventSource, eventName), false, false), nil, nil)
	
	approveBtn := slack.NewButtonBlockElement("approve", "approve", slack.NewTextBlockObject("plain_text", "Neutralize", false, false))
	approveBtn.Style = slack.StylePrimary
	denyBtn := slack.NewButtonBlockElement("deny", "deny", slack.NewTextBlockObject("plain_text", "Ignore", false, false))
	denyBtn.Style = slack.StyleDanger
	
	actionBlock := slack.NewActionBlock("actions", approveBtn, denyBtn)
	
	_, _, err := c.api.PostMessageContext(ctx, c.channelID, slack.MsgOptionBlocks(header, details, actionBlock))
	if err != nil {
		return fmt.Errorf("failed to send slack alert: %w", err)
	}
	return nil
}