// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	rdsauth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/sync/semaphore"
)

// AWSIAMAuth signs AWS RDS IAM tokens for the effective endpoint and user.
// Region and role are immutable. Empty values use the SDK's default settings.
// Create providers with NewAWSIAMAuth.
type AWSIAMAuth struct {
	region          string
	roleARN         string
	awsConfigLoader func(context.Context, string, string) (aws.Config, error)
	sema            *semaphore.Weighted
	awsCfg          *aws.Config
}

var _ Provider = (*AWSIAMAuth)(nil)

func NewAWSIAMAuth(region, roleARN string) *AWSIAMAuth {
	return &AWSIAMAuth{
		region:          region,
		roleARN:         roleARN,
		awsConfigLoader: loadDefaultAWSConfig,
		sema:            semaphore.NewWeighted(1),
	}
}

func (a *AWSIAMAuth) Name() ProviderType {
	return AWSIAM
}

func (a *AWSIAMAuth) Configure(cfg *mysql.Config) error {
	if cfg.User == "" {
		return errors.New("a database user is required")
	}

	if cfg.Passwd != "" {
		return errors.New("a password must not be set")
	}

	if cfg.Net != "tcp" {
		return errors.New("a TCP endpoint is required")
	}

	if cfg.TLS == nil && cfg.TLSConfig == "" {
		cfg.TLS = &tls.Config{}
	}

	if cfg.TLS == nil || cfg.AllowFallbackToPlaintext {
		return errors.New("TLS is required and plaintext fallback is not allowed")
	}

	cfg.AllowCleartextPasswords = true

	return nil
}

func (a *AWSIAMAuth) Authenticate(ctx context.Context, cfg *mysql.Config) error {
	awsCfg, err := a.awsConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading aws config: %w", err)
	}

	token, err := rdsauth.BuildAuthToken(ctx, cfg.Addr, awsCfg.Region, cfg.User, awsCfg.Credentials)
	if err != nil {
		return fmt.Errorf("building auth token: %w", err)
	}
	cfg.Passwd = token

	return nil
}

// awsConfig retains successful initialization so the SDK can reuse and refresh
// credentials across connections. Failed loads can be retried with a new context.
func (a *AWSIAMAuth) awsConfig(ctx context.Context) (*aws.Config, error) {
	if err := a.sema.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer a.sema.Release(1)

	if a.awsCfg != nil {
		return a.awsCfg, nil
	}

	cfg, err := a.awsConfigLoader(ctx, a.region, a.roleARN)
	if err != nil {
		return nil, err
	}

	a.awsCfg = &cfg
	return a.awsCfg, nil
}

// loadDefaultAWSConfig loads region and credentials from the AWS SDK's default chain,
// overriding the region if set and assuming roleARN if set.
// The credentials providers it returns are cached by the SDK.
func loadDefaultAWSConfig(ctx context.Context, region, roleARN string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithEC2IMDSRegion())
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading default aws config: %w", err)
	}

	if cfg.Region == "" {
		return aws.Config{}, errors.New("no aws region configured")
	}

	if roleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleARN))
	}

	return cfg, nil
}
