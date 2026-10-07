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

// Scrape `information_schema.tables` aggregated by schema.

package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const schemaSizeQuery = `
	SELECT
		TABLE_SCHEMA,
		SUM(IFNULL(DATA_LENGTH, 0) + IFNULL(INDEX_LENGTH, 0)) AS SIZE
	FROM information_schema.tables
	WHERE TABLE_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
	GROUP BY TABLE_SCHEMA
`

// Metric descriptors.
var (
	infoSchemaSchemaSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "schema_size_bytes"),
		"The size of the schema (sum of data and index lengths) from information_schema.tables.",
		[]string{"schema"}, nil,
	)
)

// ScrapeSchemaSize collects from `information_schema.tables` aggregated by schema.
type ScrapeSchemaSize struct{}

// Name of the Scraper. Should be unique.
func (ScrapeSchemaSize) Name() string {
	return informationSchema + ".schema_size"
}

// Help describes the role of the Scraper.
func (ScrapeSchemaSize) Help() string {
	return "Collect the size of each schema from information_schema.tables"
}

// Version of MySQL from which scraper is available.
func (ScrapeSchemaSize) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeSchemaSize) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) (err error) {
	rows, err := instance.getDB().QueryContext(ctx, schemaSizeQuery)
	if err != nil {
		return fmt.Errorf("query schema size: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
		var schema string
		var size float64
		if err := rows.Scan(&schema, &size); err != nil {
			return fmt.Errorf("scan schema size: %w", err)
		}
		ch <- prometheus.MustNewConstMetric(infoSchemaSchemaSizeDesc, prometheus.GaugeValue, size, schema)
	}
	return rows.Err()
}

// check interface
var _ Scraper = ScrapeSchemaSize{}
