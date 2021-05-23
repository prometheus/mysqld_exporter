// Copyright 2020 The Prometheus Authors
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

// Scrape `performance_schema.status_by_host` column information.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfStatusByHostQuery = `
	SELECT HOST, VARIABLE_NAME, VARIABLE_VALUE
	FROM performance_schema.status_by_host
	WHERE HOST IS NOT NULL;
	`

// Metric descriptors.
var (
	performanceSchemaStatusByHostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "status_by_host"),
		"The status variable value for a given host.",
		[]string{"host", "variable_name"}, nil,
	)
)

// ScrapePerfStatusByHost collects status_by_host table information.
type ScrapePerfStatusByHost struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfStatusByHost) Name() string {
	return "perf_schema.status_by_host"
}

// Help describes the role of the Scraper.
func (ScrapePerfStatusByHost) Help() string {
	return "Collect status_by_host gauges in performance_schema"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfStatusByHost) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfStatusByHost) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	statusByHostRows, err := db.QueryContext(ctx, perfStatusByHostQuery)
	if err != nil {
		return err
	}
	defer statusByHostRows.Close()

	var (
		host, variable_name string
		value               float64
	)

	for statusByHostRows.Next() {
		if err := statusByHostRows.Scan(
			&host, &variable_name, &value,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaStatusByHostDesc, prometheus.GaugeValue, value,
			host, variable_name,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfStatusByHost{}
