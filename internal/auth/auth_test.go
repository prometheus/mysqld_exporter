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
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

type testProvider struct {
	configure    func(*mysql.Config) error
	authenticate func(context.Context, *mysql.Config) error
}

func (*testProvider) Name() ProviderType                  { return "test" }
func (p *testProvider) Configure(cfg *mysql.Config) error { return p.configure(cfg) }
func (p *testProvider) Authenticate(ctx context.Context, cfg *mysql.Config) error {
	return p.authenticate(ctx, cfg)
}

func TestOpenConnectorRejectsInvalidConfiguration(t *testing.T) {
	providerErr := errors.New("connection policy rejected")
	provider := &testProvider{
		configure: func(*mysql.Config) error { return providerErr },
		authenticate: func(context.Context, *mysql.Config) error {
			t.Fatal("connector creation must not authenticate")
			return nil
		},
	}

	_, err := OpenConnector("exporter@tcp(db.example:3306)/", provider)
	if !errors.Is(err, providerErr) || !strings.Contains(err.Error(), "test: configuring authentication") {
		t.Fatalf("configuration error = %v, want wrapped provider error", err)
	}

	_, err = OpenConnector("invalid-dsn", nil)
	if err == nil || !strings.Contains(err.Error(), "parse mysql dsn") {
		t.Fatalf("invalid DSN error = %v, want parsing error", err)
	}
}

func TestOpenConnectorAuthenticatesEveryConnectionWithPrivateConfig(t *testing.T) {
	authErr := errors.New("stop before dialing")

	var (
		configured *mysql.Config
		configs    []*mysql.Config
	)

	provider := &testProvider{
		configure: func(cfg *mysql.Config) error {
			if configured != nil {
				t.Fatal("provider configured more than once per connector")
			}

			configured = cfg

			cfg.DialFunc = func(context.Context, string, string) (net.Conn, error) {
				t.Error("authentication error must prevent dialing")
				return nil, errors.New("unexpected dial")
			}

			return nil
		},
		authenticate: func(ctx context.Context, cfg *mysql.Config) error {
			configs = append(configs, cfg)

			cfg.Passwd = "connection-token"

			return authErr
		},
	}

	connector, err := OpenConnector("exporter:original@tcp(db.example:3306)/", provider)
	if err != nil {
		t.Fatal(err)
	}

	if configured == nil || len(configs) != 0 {
		t.Fatal("creating a connector must configure without authenticating")
	}

	for range 2 {
		if _, err := connector.Connect(t.Context()); !errors.Is(err, authErr) {
			t.Fatalf("Connect() error = %v, want authentication error", err)
		}
	}

	if len(configs) != 2 || configs[0] == configs[1] || configs[0] == configured || configs[1] == configured {
		t.Fatal("each connection must authenticate with its own configuration copy")
	}
	if configured.Passwd != "original" {
		t.Fatal("connection authentication changed the connector's base configuration")
	}
}
