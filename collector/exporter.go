// Copyright 2018 The Prometheus Authors
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
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric name parts.
const (
	// Subsystem(s).
	exporter = "exporter"
)

// SQL queries and parameters.
const (
	// System variable params formatting.
	// See: https://github.com/go-sql-driver/mysql#system-variables
	sessionSettingsParam = `log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27`
	timeoutParam         = `lock_wait_timeout=%d`
)

// Tunable flags.
var (
	exporterLockTimeout = kingpin.Flag(
		"exporter.lock_wait_timeout",
		"Set a lock_wait_timeout (in seconds) on the connection to avoid long metadata locking.",
	).Default("2").Int()
	slowLogFilter = kingpin.Flag(
		"exporter.log_slow_filter",
		"Add a log_slow_filter to avoid slow query logging of scrapes. NOTE: Not supported by Oracle MySQL.",
	).Default("false").Bool()
)

// metric definition
var (
	mysqlUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the MySQL server is up.",
		nil,
		nil,
	)
	mysqlScrapeCollectorSuccess = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_success"),
		"mysqld_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
	mysqlScrapeDurationSeconds = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

// Verify if Exporter implements prometheus.Collector
var _ prometheus.Collector = (*Exporter)(nil)

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	ctx      context.Context
	logger   *slog.Logger
	dsn      string
	scrapers []Scraper
	instance *instance
}

// New returns a new MySQL exporter for the provided DSN.
func New(ctx context.Context, dsn string, scrapers []Scraper, logger *slog.Logger) *Exporter {
	// Setup extra params for the DSN
	dsnParams := []string{}

	// Only set lock_wait_timeout if it is positive
	if (*exporterLockTimeout >= 0) {
		dsnParams = append(dsnParams, fmt.Sprintf(timeoutParam, *exporterLockTimeout))
	}
	
	if *slowLogFilter {
		dsnParams = append(dsnParams, sessionSettingsParam)
	}

	if strings.Contains(dsn, "?") {
		dsn = dsn + "&"
	} else {
		dsn = dsn + "?"
	}
	dsn += strings.Join(dsnParams, "&")

	return &Exporter{
		ctx:      ctx,
		logger:   logger,
		dsn:      dsn,
		scrapers: scrapers,
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- mysqlUp
	ch <- mysqlScrapeDurationSeconds
	ch <- mysqlScrapeCollectorSuccess
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	up := e.scrape(e.ctx, ch)
	ch <- prometheus.MustNewConstMetric(mysqlUp, prometheus.GaugeValue, up)
}

// scrape collects metrics from the target, returns an up metric value.
func (e *Exporter) scrape(ctx context.Context, ch chan<- prometheus.Metric) float64 {
	var err error
	scrapeTime := time.Now()
	instance, err := newInstance(e.dsn)
	if err != nil {
		e.logger.Error("Error opening connection to database", "err", err)
		return 0.0
	}
	defer instance.Close()
	e.instance = instance

	if err := instance.Ping(); err != nil {
		e.logger.Error("Error pinging mysqld", "err", err)
		return 0.0
	}

	ch <- prometheus.MustNewConstMetric(mysqlScrapeDurationSeconds, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	version := instance.versionMajorMinor

	var wg sync.WaitGroup
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		if version < scraper.Version() {
			continue
		}

		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			label := "collect." + scraper.Name()
			scrapeTime := time.Now()
			collectorSuccess := 1.0
			if err := scraper.Scrape(ctx, instance, ch, e.logger.With("scraper", scraper.Name())); err != nil {
				e.logger.Error("Error from scraper", "scraper", scraper.Name(), "target", e.getTargetFromDsn(), "err", err)
				collectorSuccess = 0.0
			}
			ch <- prometheus.MustNewConstMetric(mysqlScrapeCollectorSuccess, prometheus.GaugeValue, collectorSuccess, label)
			ch <- prometheus.MustNewConstMetric(mysqlScrapeDurationSeconds, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
	return 1.0
}

func (e *Exporter) getTargetFromDsn() string {
	// Get target from DSN.
	dsnConfig, err := mysql.ParseDSN(e.dsn)
	if err != nil {
		e.logger.Error("Error parsing DSN", "err", err)
		return ""
	}
	return dsnConfig.Addr
}
