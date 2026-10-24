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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/mysqld_exporter/internal/auth"
)

func TestAuthConfigProviderLifetime(t *testing.T) {
	// Two IAM sections exercise provider isolation; the native section needs no provider.
	const contents = `[client]
auth=aws-iam
user=exporter
password=
aws-iam-region=eu-north-1
aws-iam-role-arn=arn:aws:iam::123456789012:role/Exporter

[other]
auth=aws-iam
user=other_exporter

[native]
user=native_exporter
password=secret
`
	filename := filepath.Join(t.TempDir(), "auth.cnf")
	handler, err := NewAuthConfigHandler(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}

	reloadConfig := func(contents string) error {
		t.Helper()
		if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		return handler.ReloadConfig(filename, "localhost:3306", "", false, promslog.NewNopLogger())
	}

	// Load the initial snapshot and verify that the INI options select IAM authentication.
	if err := reloadConfig(contents); err != nil {
		t.Fatal(err)
	}
	initialSnapshot := handler.GetConfig()
	initialProvider, err := initialSnapshot.AuthProvider("client")
	if err != nil {
		t.Fatal(err)
	}
	if initialProvider == nil || initialProvider.Name() != auth.AWSIAM {
		t.Fatal("loaded section has no AWS IAM provider")
	}
	clientConfig := initialSnapshot.Sections["client"]
	if clientConfig.AWSIAMRegion != "eu-north-1" || clientConfig.AWSIAMRoleARN != "arn:aws:iam::123456789012:role/Exporter" {
		t.Fatal("IAM settings were not mapped from INI keys")
	}

	// Requests sharing a snapshot reuse providers, while different sections stay independent.
	again, err := initialSnapshot.AuthProvider("client")
	if err != nil || again != initialProvider {
		t.Fatalf("section did not retain its provider: %v", err)
	}
	other, err := initialSnapshot.AuthProvider("other")
	if err != nil || other == nil || other == initialProvider {
		t.Fatalf("sections must own separate providers: %v", err)
	}
	native, err := initialSnapshot.AuthProvider("native")
	if err != nil || native != nil {
		t.Fatalf("native authentication selected a provider: %v", err)
	}
	if _, err := initialSnapshot.AuthProvider("missing"); err == nil {
		t.Fatal("missing section must return an error")
	}

	// Changing the region publishes a fresh snapshot with a new provider.
	if err := reloadConfig(strings.ReplaceAll(contents, "eu-north-1", "us-east-1")); err != nil {
		t.Fatal(err)
	}
	reloadedSnapshot := handler.GetConfig()
	replacement, err := reloadedSnapshot.AuthProvider("client")
	if err != nil || replacement == nil || replacement == initialProvider {
		t.Fatalf("reload must create a new provider: %v", err)
	}

	// In-flight requests can still use the old snapshot and its original provider and settings.
	retained, err := initialSnapshot.AuthProvider("client")
	if err != nil || retained != initialProvider || initialSnapshot.Sections["client"].AWSIAMRegion != "eu-north-1" {
		t.Fatal("reload changed the previous snapshot used by in-flight requests")
	}

	// A failed reload must leave the last working snapshot available to new requests.
	if err := reloadConfig("[client]\nuser=exporter\nauth=unsupported\n"); err == nil {
		t.Fatal("reload accepted an unsupported provider")
	}
	if handler.GetConfig() != reloadedSnapshot {
		t.Fatal("failed reload replaced the working snapshot")
	}
}
