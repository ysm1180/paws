package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (c *Client) ListEC2Instances(ctx context.Context) ([]Instance, error) {
	resp, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var instances []Instance
	for _, reservation := range resp.Reservations {
		for _, inst := range reservation.Instances {
			name := "Unnamed Instance"
			for _, tag := range inst.Tags {
				if *tag.Key == "Name" {
					name = *tag.Value
					break
				}
			}

			privateIP := "No IP"
			if inst.PrivateIpAddress != nil {
				privateIP = *inst.PrivateIpAddress
			}

			instances = append(instances, Instance{
				ID:        *inst.InstanceId,
				Name:      name,
				PrivateIP: privateIP,
				Type:      "EC2",
			})
		}
	}

	return instances, nil
}

func (c *Client) GetBastionDisplayName(inst Instance) string {
	return fmt.Sprintf("%s (%s) - %s", inst.Name, inst.ID, inst.PrivateIP)
}
