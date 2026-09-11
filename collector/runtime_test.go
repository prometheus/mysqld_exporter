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

package collector

import (
	"context"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/prometheus/mysqld_exporter/config"
)

func TestNewRuntimeValidatesConfig(t *testing.T) {
	cfg := config.NewConfigWithDefaults()
	cfg.DataSourceName = "root@tcp(localhost:3306)/"
	if _, err := NewRuntime(cfg, promslog.NewNopLogger()); err != nil {
		t.Fatalf("expected valid config to succeed: %v", err)
	}

	cfg.DataSourceName = ""
	if _, err := NewRuntime(cfg, promslog.NewNopLogger()); err == nil {
		t.Fatal("expected runtime construction to revalidate config")
	}
}

func TestNewRuntimeCollectors(t *testing.T) {
	cfg := config.NewConfigWithDefaults()
	cfg.DataSourceName = "root@tcp(localhost:3306)/"

	runtime, err := NewRuntime(cfg, promslog.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected runtime error: %v", err)
	}
	if got := len(runtime.Collectors()); got != 1 {
		t.Fatalf("unexpected collector count: got %d, want 1", got)
	}
}

func TestRuntimeShutdownCancelsContext(t *testing.T) {
	cfg := config.NewConfigWithDefaults()
	cfg.DataSourceName = "root@tcp(localhost:3306)/"

	runtime, err := NewRuntime(cfg, promslog.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected runtime error: %v", err)
	}

	select {
	case <-runtime.exporter.ctx.Done():
		t.Fatal("runtime context canceled before shutdown")
	default:
	}

	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	select {
	case <-runtime.exporter.ctx.Done():
	default:
		t.Fatal("runtime context was not canceled by shutdown")
	}

	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown should be idempotent: %v", err)
	}
}

func TestNewRuntimeWithContextPropagatesParentCancellation(t *testing.T) {
	cfg := config.NewConfigWithDefaults()
	cfg.DataSourceName = "root@tcp(localhost:3306)/"
	ctx, cancel := context.WithCancel(t.Context())

	runtime, err := NewRuntimeWithContext(ctx, cfg, promslog.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected runtime error: %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Shutdown(t.Context())
	})

	cancel()
	select {
	case <-runtime.exporter.ctx.Done():
	default:
		t.Fatal("parent cancellation did not reach runtime context")
	}
}

func TestEnabledScrapersUsesConfig(t *testing.T) {
	cfg := config.NewConfigWithDefaults()
	cfg.Collectors = map[string]bool{
		"heartbeat": true,
	}
	cfg.Heartbeat.Database = "hb_db"
	cfg.Heartbeat.Table = "hb_table"
	cfg.Heartbeat.UTC = true

	scrapers := EnabledScrapers(cfg)
	if len(scrapers) != 1 {
		t.Fatalf("unexpected scraper count: got %d, want 1", len(scrapers))
	}
	if scrapers[0].Name() != "heartbeat" {
		t.Fatalf("unexpected scraper: got %s, want heartbeat", scrapers[0].Name())
	}
}
