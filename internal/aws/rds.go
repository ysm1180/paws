package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"
)

func (c *Client) ListRDSInstances(ctx context.Context) ([]Instance, error) {
	resp, err := c.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, err
	}

	var instances []Instance
	for _, db := range resp.DBInstances {
		if db.Endpoint == nil {
			continue
		}

		instances = append(instances, Instance{
			ID:       *db.DBInstanceIdentifier,
			Name:     *db.DBInstanceIdentifier,
			Endpoint: *db.Endpoint.Address,
			Port:     int(*db.Endpoint.Port),
			Type:     "RDS Instance",
		})
	}

	return instances, nil
}
