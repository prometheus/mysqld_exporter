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
	"errors"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/mysqld_exporter/config"
)

type Runtime struct {
	cancel   context.CancelFunc
	exporter *Exporter
}

func NewRuntime(cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	return NewRuntimeWithContext(context.Background(), cfg, logger)
}

// NewRuntimeWithContext creates a Runtime whose lifetime is also bounded by ctx.
// Long-lived embedded consumers should use NewRuntime and call Runtime.Shutdown.
func NewRuntimeWithContext(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	runtimeCtx, cancel := context.WithCancel(ctx)

	return &Runtime{
		cancel: cancel,
		exporter: New(
			runtimeCtx,
			cfg.DataSourceName,
			EnabledScrapers(cfg),
			logger,
			EnableLockWaitTimeout(cfg.EnableExporterLockWaitTimeout),
			SetLockWaitTimeout(cfg.ExporterLockWaitTimeoutSeconds),
			SetSlowLogFilter(cfg.SlowLogFilter),
			SetQueryTimeout(cfg.ExporterQueryTimeout),
			SetMaxOpenConns(cfg.ExporterMaxOpenConns),
		),
	}, nil
}

// Shutdown cancels in-flight collection work owned by the Runtime.
func (r *Runtime) Shutdown(context.Context) error {
	r.cancel()
	return nil
}

func (r *Runtime) Collectors() []prometheus.Collector {
	if r == nil || r.exporter == nil {
		return nil
	}
	return []prometheus.Collector{r.exporter}
}

func EnabledScrapers(cfg config.Config) []Scraper {
	scrapers := AllScrapers(cfg)
	enabled := make([]Scraper, 0, len(scrapers))
	for _, scraper := range scrapers {
		if cfg.Collectors[scraper.Name()] {
			enabled = append(enabled, scraper)
		}
	}
	return enabled
}

func AllScrapers(cfg config.Config) []Scraper {
	emptyConfig := config.EmptyConfig{}

	return []Scraper{
		ScrapeGlobalStatus(emptyConfig),
		ScrapeGlobalVariables(emptyConfig),
		ScrapeSlaveStatus(emptyConfig),
		ScrapeProcesslist(cfg.InfoSchemaProcesslist),
		ScrapeUser(cfg.MysqlUser),
		ScrapeTableSchema(cfg.InfoSchemaTables),
		ScrapeInfoSchemaInnodbTablespaces(emptyConfig),
		ScrapeInnodbMetrics(emptyConfig),
		ScrapeAutoIncrementColumns(emptyConfig),
		ScrapeBinlogSize(emptyConfig),
		ScrapePerfTableIOWaits(emptyConfig),
		ScrapePerfIndexIOWaits(emptyConfig),
		ScrapePerfTableLockWaits(emptyConfig),
		ScrapePerfEventsStatements(cfg.PerfSchemaEventsStatements),
		ScrapePerfEventsStatementsSum(emptyConfig),
		ScrapePerfEventsWaits(emptyConfig),
		ScrapePerfFileEvents(emptyConfig),
		ScrapePerfFileInstances(cfg.PerfSchemaFileInstances),
		ScrapePerfMemoryEvents(cfg.PerfSchemaMemoryEvents),
		ScrapePerfReplicationGroupMembers(emptyConfig),
		ScrapePerfReplicationGroupMemberStats(emptyConfig),
		ScrapePerfReplicationApplierStatsByWorker(emptyConfig),
		ScrapeSysUserSummary(emptyConfig),
		ScrapeUserStat(emptyConfig),
		ScrapeClientStat(emptyConfig),
		ScrapeTableStat(emptyConfig),
		ScrapeSchemaStat(emptyConfig),
		ScrapeInnodbCmp(emptyConfig),
		ScrapeInnodbCmpMem(emptyConfig),
		ScrapeQueryResponseTime(emptyConfig),
		ScrapeEngineTokudbStatus(emptyConfig),
		ScrapeEngineInnodbStatus(emptyConfig),
		ScrapeHeartbeat(cfg.Heartbeat),
		ScrapeSlaveHosts(emptyConfig),
		ScrapeReplicaHost(emptyConfig),
		ScrapeRocksDBPerfContext(emptyConfig),
	}
}
