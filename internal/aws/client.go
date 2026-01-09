package aws

import (
	"context"

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

	return &Client{
		EC2:         ec2.NewFromConfig(cfg),
		RDS:         rds.NewFromConfig(cfg),
		ElastiCache: elasticache.NewFromConfig(cfg),
		SSM:         ssm.NewFromConfig(cfg),
		Region:      cfg.Region,
	}, nil
}
