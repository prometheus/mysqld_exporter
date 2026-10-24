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

// Scrape `performance_schema.host_cache`.

package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/mysqld_exporter/config"
)

// The host cache is only used when host_cache_size > 0 and skip_name_resolve
// is OFF. Otherwise the table is empty, the server does not block hosts, and
// all values are 0. On MariaDB, performance_schema=ON is also required: when
// it is OFF, the table is empty, but MariaDB can still block hosts.
const perfHostCacheQuery = `
	SELECT
		IFNULL(SUM(SUM_CONNECT_ERRORS), 0),
		IFNULL(MAX(SUM_CONNECT_ERRORS), 0),
		IFNULL(SUM(COUNT_HOST_BLOCKED_ERRORS), 0),
		COUNT(*)
	  FROM performance_schema.host_cache
	`

// All values are gauges, not counters, because they can go down: a successful
// connection resets the error count of a host, and TRUNCATE
// performance_schema.host_cache (or FLUSH HOSTS before MySQL 8.4) and cache
// eviction remove rows.
var (
	performanceSchemaHostCacheConnectErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "host_cache_connect_errors"),
		"The sum of connection errors that count toward max_connect_errors, for all hosts in the host cache.",
		[]string{}, nil,
	)
	performanceSchemaHostCacheConnectErrorsMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "host_cache_connect_errors_max"),
		"The highest number of connection errors of a single host. A host is blocked when it reaches max_connect_errors.",
		[]string{}, nil,
	)
	performanceSchemaHostCacheHostBlockedErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "host_cache_host_blocked_errors"),
		"The sum of connection attempts that were refused because the host was blocked.",
		[]string{}, nil,
	)
	performanceSchemaHostCacheHostsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "host_cache_hosts"),
		"The number of hosts in the host cache.",
		[]string{}, nil,
	)
)

// ScrapePerfHostCache collects from `performance_schema.host_cache`.
type ScrapePerfHostCache config.EmptyConfig

// Name of the Scraper. Should be unique.
func (ScrapePerfHostCache) Name() string {
	return "perf_schema.host_cache"
}

// Help describes the role of the Scraper.
func (ScrapePerfHostCache) Help() string {
	return "Collect aggregated metrics from performance_schema.host_cache"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfHostCache) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfHostCache) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()

	var connectErrors, connectErrorsMax, hostBlockedErrors, hosts float64
	err := db.QueryRowContext(ctx, perfHostCacheQuery).Scan(
		&connectErrors, &connectErrorsMax, &hostBlockedErrors, &hosts,
	)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		performanceSchemaHostCacheConnectErrorsDesc, prometheus.GaugeValue, connectErrors,
	)
	ch <- prometheus.MustNewConstMetric(
		performanceSchemaHostCacheConnectErrorsMaxDesc, prometheus.GaugeValue, connectErrorsMax,
	)
	ch <- prometheus.MustNewConstMetric(
		performanceSchemaHostCacheHostBlockedErrorsDesc, prometheus.GaugeValue, hostBlockedErrors,
	)
	ch <- prometheus.MustNewConstMetric(
		performanceSchemaHostCacheHostsDesc, prometheus.GaugeValue, hosts,
	)
	return nil
}

// check interface
var _ Scraper = ScrapePerfHostCache{}
