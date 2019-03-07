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

// Scrape `information_schema.table_statistics` grouped by regex.

package collector

import (
	"context"
	"database/sql"

	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

// TODO: Can be reused?
const tableStatFilteredQuery = `
		SELECT
		  TABLE_SCHEMA,
		  TABLE_NAME,
		  ROWS_READ,
		  ROWS_CHANGED,
		  ROWS_CHANGED_X_INDEXES
		  FROM information_schema.table_statistics
		`

// Metric descriptors. TODO: Update descriptions and names
var (
	infoSchemaTableStatsFilteredRowsReadDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_read_total"),
		"The number of rows read from the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsFilteredRowsChangedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_total"),
		"The number of rows changed in the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsFilteredRowsChangedXIndexesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_x_indexes_total"),
		"The number of rows changed in the table, multiplied by the number of indexes changed.",
		[]string{"schema", "table"}, nil,
	)
)

// Configuration
var (
	regex = kingpin.Flag(
		"collect.info_schema_tablestats_filtered.regex",
		"Regex with capture groups for renaming the talbes",
	).Default("(.*)").String()

	substitution = kingpin.Flag(
		"collect.info_schema_tablestats_filtered.substitution",
		"Substitution string to apply to the table name",
	).Default("$1").String()
)

// ScrapeTableStatFiltered collects from `information_schema.table_statistics`.
type ScrapeTableStatFiltered struct{}

// Name of the Scraper. Should be unique.
func (ScrapeTableStatFiltered) Name() string {
	return "info_schema.tablestatsfiltered"
}

// Help describes the role of the Scraper.
func (ScrapeTableStatFiltered) Help() string {
	return "If running with userstat=1, set to true to collect table statistics"
}

// Version of MySQL from which scraper is available.
func (ScrapeTableStatFiltered) Version() float64 {
	return 5.1
}

type tableStats struct {
	schema              string
	name                string
	rowsRead            uint64
	rowsChanged         uint64
	rowsChangedXIndexes uint64
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeTableStatFiltered) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	var varName, varVal string
	err := db.QueryRowContext(ctx, userstatCheckQuery).Scan(&varName, &varVal)
	if err != nil {
		log.Debugln("Detailed table stats are not available.")
		return nil
	}
	if varVal == "OFF" {
		log.Debugf("MySQL @@%s is OFF.", varName)
		return nil
	}

	informationSchemaTableStatisticsRows, err := db.QueryContext(ctx, tableStatFilteredQuery)
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

	var aggregatedStats = make(map[string]tableStats)

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

		tableNameRegex := regexp.MustCompile(*regex)

		tableName := tableNameRegex.ReplaceAllString(tableName, *substitution)

		stats, found := aggregatedStats[tableSchema+"."+tableName]

		if !found {
			stats = tableStats{tableSchema, tableName, 0, 0, 0}
		}

		stats.rowsChanged += rowsChanged
		stats.rowsChangedXIndexes += rowsChangedXIndexes
		stats.rowsRead += rowsRead

		aggregatedStats[tableSchema+"."+tableName] = stats
	}

	for _, table := range aggregatedStats {

		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsFilteredRowsReadDesc, prometheus.CounterValue, float64(table.rowsRead),
			table.schema, table.name,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsFilteredRowsChangedDesc, prometheus.CounterValue, float64(table.rowsChanged),
			table.schema, table.name,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsFilteredRowsChangedXIndexesDesc, prometheus.CounterValue, float64(table.rowsChangedXIndexes),
			table.schema, table.name,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapeTableStatFiltered{}
