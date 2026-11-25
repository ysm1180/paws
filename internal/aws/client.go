package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Client struct {
	EC2         *ec2.Client
	RDS         *rds.Client
	ElastiCache *elasticache.Client
	SSM         *ssm.Client
	Region      string
	Profile     string
}

func NewClient(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return newClientFromConfig(cfg), nil
}

// NewClientWithRegion builds a client pinned to an explicit region,
// ignoring the region from the default config chain.
func NewClientWithRegion(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return newClientFromConfig(cfg), nil
}

func newClientFromConfig(cfg aws.Config) *Client {
	return &Client{
		EC2:         ec2.NewFromConfig(cfg),
		RDS:         rds.NewFromConfig(cfg),
		ElastiCache: elasticache.NewFromConfig(cfg),
		SSM:         ssm.NewFromConfig(cfg),
		Region:      cfg.Region,
	}
}
