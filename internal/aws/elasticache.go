package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
)

func (c *Client) ListElastiCacheInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance

	rgResp, err := c.ElastiCache.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{})
	if err != nil {
		return nil, err
	}

	for _, rg := range rgResp.ReplicationGroups {
		if len(rg.NodeGroups) == 0 {
			continue
		}

		nodeGroup := rg.NodeGroups[0]
		if nodeGroup.PrimaryEndpoint == nil {
			continue
		}

		desc := ""
		if rg.Description != nil {
			desc = *rg.Description
		}

		instances = append(instances, Instance{
			ID:          *rg.ReplicationGroupId,
			Name:        *rg.ReplicationGroupId,
			Endpoint:    *nodeGroup.PrimaryEndpoint.Address,
			Port:        int(*nodeGroup.PrimaryEndpoint.Port),
			Type:        "ElastiCache (Redis/Valkey)",
			Description: desc,
		})
	}

	ccResp, err := c.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	for _, cc := range ccResp.CacheClusters {
		if cc.ReplicationGroupId != nil {
			continue
		}

		if len(cc.CacheNodes) == 0 || cc.CacheNodes[0].Endpoint == nil {
			continue
		}

		node := cc.CacheNodes[0]
		engine := "unknown"
		if cc.Engine != nil {
			engine = *cc.Engine
		}

		instances = append(instances, Instance{
			ID:       *cc.CacheClusterId,
			Name:     *cc.CacheClusterId,
			Endpoint: *node.Endpoint.Address,
			Port:     int(*node.Endpoint.Port),
			Type:     fmt.Sprintf("ElastiCache (%s)", engine),
		})
	}

	return instances, nil
}
