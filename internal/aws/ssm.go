package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type SessionInfo struct {
	SessionID    string
	StreamURL    string
	TokenValue   string
	Region       string
	BastionID    string
}

func (c *Client) StartPortForwardingSession(ctx context.Context, bastionID, remoteHost string, localPort, remotePort int) (*SessionInfo, error) {
	resp, err := c.SSM.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(bastionID),
		DocumentName: aws.String("AWS-StartPortForwardingSessionToRemoteHost"),
		Parameters: map[string][]string{
			"localPortNumber": {intToStr(localPort)},
			"host":            {remoteHost},
			"portNumber":      {intToStr(remotePort)},
		},
	})
	if err != nil {
		return nil, err
	}

	return &SessionInfo{
		SessionID:  *resp.SessionId,
		StreamURL:  *resp.StreamUrl,
		TokenValue: *resp.TokenValue,
		Region:     c.Region,
		BastionID:  bastionID,
	}, nil
}

func (c *Client) TerminateSession(ctx context.Context, sessionID string) error {
	_, err := c.SSM.TerminateSession(ctx, &ssm.TerminateSessionInput{
		SessionId: aws.String(sessionID),
	})
	return err
}

func intToStr(n int) string {
	return fmt.Sprintf("%d", n)
}
