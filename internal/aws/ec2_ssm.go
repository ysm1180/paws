package aws

import (
	"context"
	"fmt"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func (c *Client) ListSsmManagedEC2Instances(ctx context.Context) ([]Instance, error) {
	info, err := c.SSM.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{})
	if err != nil {
		return nil, fmt.Errorf("describe instance info: %w", err)
	}

	pingByID := make(map[string]string, len(info.InstanceInformationList))
	platformByID := make(map[string]string, len(info.InstanceInformationList))
	for _, i := range info.InstanceInformationList {
		if i.InstanceId == nil {
			continue
		}
		pingByID[*i.InstanceId] = string(i.PingStatus)
		platformByID[*i.InstanceId] = string(i.PlatformType)
	}

	resp, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: awsv2.String("instance-state-name"), Values: []string{"running"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	var out []Instance
	for _, r := range resp.Reservations {
		for _, inst := range r.Instances {
			if inst.InstanceId == nil {
				continue
			}
			id := *inst.InstanceId
			ping, ok := pingByID[id]
			if !ok || ping != "Online" {
				continue
			}
			name := "Unnamed Instance"
			for _, tag := range inst.Tags {
				if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
					name = *tag.Value
					break
				}
			}
			privateIP := "No IP"
			if inst.PrivateIpAddress != nil {
				privateIP = *inst.PrivateIpAddress
			}
			out = append(out, Instance{
				ID:            id,
				Name:          name,
				PrivateIP:     privateIP,
				Type:          "EC2",
				Platform:      platformByID[id],
				SsmPingStatus: ping,
			})
		}
	}
	return out, nil
}
