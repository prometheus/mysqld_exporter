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

// Scrape `information_schema.table_statistics`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

const tableStatQuery = `
		SELECT
		  TABLE_SCHEMA,
		  TABLE_NAME,
		  ROWS_READ,
		  ROWS_CHANGED,
		  ROWS_CHANGED_X_INDEXES
		  FROM information_schema.table_statistics
		`

// Metric descriptors.
var (
	infoSchemaTableStatsRowsReadDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_read_total"),
		"The number of rows read from the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsRowsChangedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_total"),
		"The number of rows changed in the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsRowsChangedXIndexesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_x_indexes_total"),
		"The number of rows changed in the table, multiplied by the number of indexes changed.",
		[]string{"schema", "table"}, nil,
	)
)

// ScrapeTableStat collects from `information_schema.table_statistics`.
type ScrapeTableStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeTableStat) Name() string {
	return "info_schema.tablestats"
}

// Help describes the role of the Scraper.
func (ScrapeTableStat) Help() string {
	return "If running with userstat=1, set to true to collect table statistics"
}

// Version of MySQL from which scraper is available.
func (ScrapeTableStat) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeTableStat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var varName, varVal string
	err := db.QueryRowContext(ctx, userstatCheckQuery).Scan(&varName, &varVal)
	if err != nil {
		level.Debug(logger).Log("msg", "Detailed table stats are not available.")
		return nil
	}
	if varVal == "OFF" {
		level.Debug(logger).Log("msg", "MySQL variable is OFF.", "var", varName)
		return nil
	}

	informationSchemaTableStatisticsRows, err := db.QueryContext(ctx, tableStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaTableStatisticsRows.Close()

	var (
		tableSchema         string
		tableName           string
		rowsRead            uint64
		rowsChanged         uint64
		rowsChangedXIndexes uint64
	)

	for informationSchemaTableStatisticsRows.Next() {
		err = informationSchemaTableStatisticsRows.Scan(
			&tableSchema,
			&tableName,
			&rowsRead,
			&rowsChanged,
			&rowsChangedXIndexes,
		)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsReadDesc, prometheus.CounterValue, float64(rowsRead),
			tableSchema, tableName,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsChangedDesc, prometheus.CounterValue, float64(rowsChanged),
			tableSchema, tableName,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsChangedXIndexesDesc, prometheus.CounterValue, float64(rowsChangedXIndexes),
			tableSchema, tableName,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapeTableStat{}
