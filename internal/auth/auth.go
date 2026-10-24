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

// Package auth prepares MySQL connections for password or token authentication.
package auth

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

type ProviderType string

const AWSIAM ProviderType = "aws-iam"

// Provider prepares authentication for a connector and each physical connection.
type Provider interface {
	// Name of the provider for errors.
	Name() ProviderType

	// Configure applies provider policy to a parsed, normalized MySQL config.
	Configure(cfg *mysql.Config) error

	// Authenticate runs before each physical connection on the driver.
	Authenticate(ctx context.Context, cfg *mysql.Config) error
}

// OpenConnector creates a connector without connecting or retrieving credentials.
// A nil provider uses native MySQL authentication without provider hooks.
// The MySQL driver performs the wire authentication handshake.
func OpenConnector(dsn string, provider Provider) (driver.Connector, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse mysql dsn: %w", err)
	}

	if provider != nil {
		if err := provider.Configure(cfg); err != nil {
			return nil, fmt.Errorf("%s: configuring authentication: %w", provider.Name(), err)
		}

		if err := cfg.Apply(mysql.BeforeConnect(provider.Authenticate)); err != nil {
			return nil, fmt.Errorf("%s: applying authentication callback: %w", provider.Name(), err)
		}
	}

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("new driver connector: %w", err)
	}

	return connector, nil
}
