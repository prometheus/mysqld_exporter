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
	"database/sql"
	"regexp"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric name parts.
const (
	// Subsystem(s).
	exporter = "exporter"
)

// SQL Queries.
const (
	versionQuery = `SELECT @@version`
)

var (
	versionRE = regexp.MustCompile(`^\d+\.\d+`)
)

// Metric descriptors.
var (
	scrapeDurationDesc = prometheus.NewDesc(
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
	logger   log.Logger
	db       *sql.DB
	scrapers []Scraper
	metrics  Metrics
}

// New returns a new MySQL exporter for the provided DSN.
func New(ctx context.Context, db *sql.DB, metrics Metrics, scrapers []Scraper, logger log.Logger) *Exporter {
	return &Exporter{
		ctx:      ctx,
		logger:   logger,
		db:       db,
		scrapers: scrapers,
		metrics:  metrics,
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.metrics.TotalScrapes.Desc()
	ch <- e.metrics.Error.Desc()
	e.metrics.ScrapeErrors.Describe(ch)
	ch <- e.metrics.MySQLUp.Desc()
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(e.ctx, ch)

	ch <- e.metrics.TotalScrapes
	ch <- e.metrics.Error
	e.metrics.ScrapeErrors.Collect(ch)
	ch <- e.metrics.MySQLUp
}

func (e *Exporter) scrape(ctx context.Context, ch chan<- prometheus.Metric) {
	e.metrics.TotalScrapes.Inc()

	scrapeTime := time.Now()
	if err := e.db.PingContext(ctx); err != nil {
		// BUG(arvenil): PMM-2726: When PingContext returns with context deadline exceeded
		// the subsequent call will return `bad connection`.
		// https://github.com/go-sql-driver/mysql/issues/858
		// The PingContext is called second time as a workaround for this issue.
		if err = e.db.PingContext(ctx); err != nil {
			level.Error(e.logger).Log("msg", "Error pinging mysqld", "err", err)
			e.metrics.MySQLUp.Set(0)
			e.metrics.Error.Set(1)
			return
		}
	}

	e.metrics.MySQLUp.Set(1)
	e.metrics.Error.Set(0)

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	version := getMySQLVersion(e.db, e.logger)
	var wg sync.WaitGroup
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		if version < scraper.Version() {
			continue
		}

		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()

			defer pprof.SetGoroutineLabels(ctx)
			scrapeCtx := pprof.WithLabels(ctx, pprof.Labels("scraper", scraper.Name()))
			pprof.SetGoroutineLabels(scrapeCtx)

			label := "collect." + scraper.Name()
			scrapeTime := time.Now()
			if err := scraper.Scrape(scrapeCtx, e.db, ch, log.With(e.logger, "scraper", scraper.Name())); err != nil {
				level.Error(e.logger).Log("msg", "Error from scraper", "scraper", scraper.Name(), "err", err)
				e.metrics.ScrapeErrors.WithLabelValues(label).Inc()
				e.metrics.Error.Set(1)
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
}

func getMySQLVersion(db *sql.DB, logger log.Logger) float64 {
	var versionStr string
	var versionNum float64
	if err := db.QueryRow(versionQuery).Scan(&versionStr); err == nil {
		versionNum, _ = strconv.ParseFloat(versionRE.FindString(versionStr), 64)
	} else {
		level.Debug(logger).Log("msg", "Error querying version", "err", err)
	}
	// If we can't match/parse the version, set it some big value that matches all versions.
	if versionNum == 0 {
		level.Debug(logger).Log("msg", "Error parsing version string", "version", versionStr)
		versionNum = 999
	}
	return versionNum
}

// Metrics represents exporter metrics which values can be carried between http requests.
type Metrics struct {
	TotalScrapes prometheus.Counter
	ScrapeErrors *prometheus.CounterVec
	Error        prometheus.Gauge
	MySQLUp      prometheus.Gauge
}

// NewMetrics creates new Metrics instance.
func NewMetrics(resolution string) Metrics {
	subsystem := exporter
	if resolution != "" {
		subsystem = exporter + "_" + resolution
	}
	return Metrics{
		TotalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrapes_total",
			Help:      "Total number of times MySQL was scraped for metrics.",
		}),
		ScrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a MySQL.",
		}, []string{"collector"}),
		Error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from MySQL resulted in an error (1 for error, 0 for success).",
		}),
		MySQLUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the MySQL server is up.",
		}),
	}
}
