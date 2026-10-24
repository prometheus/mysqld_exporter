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
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/go-sql-driver/mysql"
)

func TestAWSIAMConfigure(t *testing.T) {
	tests := []struct {
		name       string
		dsn        string
		wantErr    string
		skipVerify bool
	}{
		{name: "verified TLS by default", dsn: "exporter@tcp(db.example:3306)/"},
		{name: "explicit TLS", dsn: "exporter@tcp(db.example:3306)/?tls=true"},
		{name: "explicit skip verification", dsn: "exporter@tcp(db.example:3306)/?tls=skip-verify", skipVerify: true},
		{name: "missing user", dsn: "tcp(db.example:3306)/", wantErr: "a database user is required"},
		{name: "static password", dsn: "exporter:secret@tcp(db.example:3306)/", wantErr: "a password must not be set"},
		{name: "unix socket", dsn: "exporter@unix(/tmp/mysql.sock)/", wantErr: "a TCP endpoint is required"},
		{name: "TLS disabled", dsn: "exporter@tcp(db.example:3306)/?tls=false", wantErr: "TLS is required"},
		{name: "plaintext fallback", dsn: "exporter@tcp(db.example:3306)/?tls=preferred", wantErr: "plaintext fallback is not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := mysql.ParseDSN(tt.dsn)
			if err != nil {
				t.Fatal(err)
			}

			provider := NewAWSIAMAuth("", "")
			provider.awsConfigLoader = func(context.Context, string, string) (aws.Config, error) {
				t.Fatal("Configure must not load AWS configuration")
				return aws.Config{}, nil
			}

			err = provider.Configure(cfg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Configure() error = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if cfg.TLS == nil || cfg.TLS.InsecureSkipVerify != tt.skipVerify {
				t.Fatalf("unexpected TLS configuration: %+v", cfg.TLS)
			}

			if !cfg.AllowCleartextPasswords || cfg.AllowFallbackToPlaintext {
				t.Fatal("IAM requires cleartext plugin support over TLS without plaintext fallback")
			}
		})
	}

	t.Run("preserves custom TLS settings", func(t *testing.T) {
		cfg := mysql.NewConfig()
		cfg.User = "exporter"
		cfg.Net = "tcp"
		cfg.TLS = &tls.Config{ServerName: "db.example", MinVersion: tls.VersionTLS13}

		if err := NewAWSIAMAuth("", "").Configure(cfg); err != nil {
			t.Fatal(err)
		}

		if cfg.TLS.ServerName != "db.example" || cfg.TLS.MinVersion != tls.VersionTLS13 {
			t.Fatal("Configure replaced custom TLS settings")
		}
	})
}

func TestAWSIAMAuthenticateSignsEachConnection(t *testing.T) {
	const region = "eu-north-1"
	const roleARN = "arn:aws:iam::123456789012:role/Exporter"

	provider := NewAWSIAMAuth(region, roleARN)

	loads := 0
	retrievals := 0
	provider.awsConfigLoader = func(ctx context.Context, gotRegion, gotRole string) (aws.Config, error) {
		loads++
		return aws.Config{
			Region: region,
			Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				retrievals++
				return aws.Credentials{
					AccessKeyID:     fmt.Sprintf("test-key-%d", retrievals),
					SecretAccessKey: "test-secret",
					SessionToken:    "test-session",
				}, nil
			}),
		}, nil
	}

	for i, cfg := range []*mysql.Config{
		{Addr: "first.example:3306", User: "first_exporter"},
		{Addr: "second.example:3307", User: "second_exporter"},
	} {
		if err := provider.Authenticate(t.Context(), cfg); err != nil {
			t.Fatal(err)
		}

		token := parseAuthToken(t, cfg.Passwd)
		if token.Host != cfg.Addr || token.Query().Get("DBUser") != cfg.User {
			t.Fatalf("token does not match connection endpoint %q and user %q", cfg.Addr, cfg.User)
		}

		scope := token.Query().Get("X-Amz-Credential")
		if !strings.HasPrefix(scope, fmt.Sprintf("test-key-%d/", i+1)) || !strings.HasSuffix(scope, "/"+region+"/rds-db/aws4_request") {
			t.Fatalf("unexpected signing credential scope: %s", scope)
		}
		if token.Query().Get("X-Amz-Security-Token") != "test-session" || token.Query().Get("X-Amz-Signature") == "" {
			t.Fatal("token is missing temporary credentials or its signature")
		}
	}

	if loads != 1 {
		t.Fatalf("configuration loads = %d, want 1 across connections", loads)
	}
}

func TestAWSIAMRetriesFailedInitialization(t *testing.T) {
	provider := testAWSIAMProvider(credentials.NewStaticCredentialsProvider("test-key", "test-secret", ""))
	defaultLoader := provider.awsConfigLoader

	loadErr := errors.New("configuration temporarily unavailable")
	loads := 0
	provider.awsConfigLoader = func(ctx context.Context, region, roleARN string) (aws.Config, error) {
		loads++
		if loads == 1 {
			return aws.Config{}, loadErr
		}
		return defaultLoader(ctx, region, roleARN)
	}

	cfg := &mysql.Config{
		User: "exporter",
		Addr: "db.example:3306",
	}
	if err := provider.Authenticate(t.Context(), cfg); !errors.Is(err, loadErr) {
		t.Fatalf("first authentication error = %v, want loader error", err)
	}

	for range 2 {
		if err := provider.Authenticate(t.Context(), cfg); err != nil {
			t.Fatal(err)
		}
	}

	if loads != 2 {
		t.Fatalf("configuration loads = %d, want failed load plus one successful load", loads)
	}
}

func TestAWSIAMCredentialFailureDoesNotReplacePassword(t *testing.T) {
	credentialErr := errors.New("credentials temporarily unavailable")
	calls := 0
	provider := testAWSIAMProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		calls++
		if calls == 1 {
			return aws.Credentials{}, credentialErr
		}
		return aws.Credentials{AccessKeyID: "test-key", SecretAccessKey: "test-secret"}, nil
	}))

	const passwd = "initial-passwd"

	cfg := &mysql.Config{
		User:   "exporter",
		Addr:   "db.example:3306",
		Passwd: passwd,
	}
	if err := provider.Authenticate(t.Context(), cfg); !errors.Is(err, credentialErr) {
		t.Fatalf("authentication error = %v, want credential error", err)
	}
	if cfg.Passwd != passwd {
		t.Fatal("failed authentication replaced the connection password")
	}

	if err := provider.Authenticate(t.Context(), cfg); err != nil {
		t.Fatalf("credential retrieval did not recover: %v", err)
	}
}

func testAWSIAMProvider(creds aws.CredentialsProvider) *AWSIAMAuth {
	provider := NewAWSIAMAuth("eu-north-1", "")
	provider.awsConfigLoader = func(context.Context, string, string) (aws.Config, error) {
		return aws.Config{Region: "eu-north-1", Credentials: creds}, nil
	}
	return provider
}

func parseAuthToken(t *testing.T, token string) *url.URL {
	t.Helper()
	parsed, err := url.Parse("https://" + token)
	if err != nil {
		t.Fatalf("invalid auth token: %v", err)
	}
	return parsed
}
