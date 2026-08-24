package service

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWSService struct {
	region string
}

func NewAWSService(region string) *AWSService {
	return &AWSService{region: region}
}

type TempCreds struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func (s *AWSService) AssumeRole(ctx context.Context, roleArn, sessionName string) (*TempCreds, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.region))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	}

	result, err := stsClient.AssumeRole(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	return &TempCreds{
		AccessKeyID:     *result.Credentials.AccessKeyId,
		SecretAccessKey: *result.Credentials.SecretAccessKey,
		SessionToken:    *result.Credentials.SessionToken,
	}, nil
}
