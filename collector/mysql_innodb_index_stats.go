// Copyright 2026 The Prometheus Authors
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

// Scrape `mysql.innodb_index_stats`.

package collector

import (
	"context"
	"log/slog"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/mysqld_exporter/config"
)

const (
	innodbIndexStatsSubsystem = "innodb_index_stats"

	innodbPageSizeQuery = `SELECT @@innodb_page_size`

	// innodbIndexStatsQuery reads the persistent index statistics. The %s is
	// replaced with the database filter.
	innodbIndexStatsQuery = `
		SELECT
		    database_name,
		    table_name,
		    index_name,
		    stat_name,
		    stat_value,
		    ifnull(stat_description, '') AS stat_description,
		    UNIX_TIMESTAMP(last_update) AS last_update
		  FROM mysql.innodb_index_stats
		  WHERE %s
		`

	innodbIndexStatsSystemSchemasFilter = `database_name NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')`
)

// Metric descriptors.
var (
	innodbIndexStatsLabels = []string{"schema", "table", "index"}

	innodbIndexStatsSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, innodbIndexStatsSubsystem, "size_bytes"),
		"Estimated size of the index, from the number of pages InnoDB recorded in its persistent statistics.",
		innodbIndexStatsLabels, nil,
	)
	innodbIndexStatsLeafSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, innodbIndexStatsSubsystem, "leaf_size_bytes"),
		"Estimated size of the leaf pages of the index, from InnoDB's persistent statistics.",
		innodbIndexStatsLabels, nil,
	)
	innodbIndexStatsDistinctValuesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, innodbIndexStatsSubsystem, "distinct_values"),
		"Estimated number of distinct values of the leading index columns listed in the columns label, from InnoDB's persistent statistics.",
		append(innodbIndexStatsLabels, "columns"), nil,
	)
	innodbIndexStatsLastUpdateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, innodbIndexStatsSubsystem, "last_update_timestamp_seconds"),
		"When InnoDB last recalculated the persistent statistics of the index.",
		innodbIndexStatsLabels, nil,
	)
)

// ScrapeInnodbIndexStats collects from `mysql.innodb_index_stats`.
type ScrapeInnodbIndexStats config.InnodbIndexStatsConfig

// Name of the Scraper. Should be unique.
func (ScrapeInnodbIndexStats) Name() string {
	return "mysql.innodb_index_stats"
}

// Help describes the role of the Scraper.
func (ScrapeInnodbIndexStats) Help() string {
	return "Collect per index size and cardinality estimates from mysql.innodb_index_stats"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbIndexStats) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (s ScrapeInnodbIndexStats) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()

	var pageSize float64
	if err := db.QueryRowContext(ctx, innodbPageSizeQuery).Scan(&pageSize); err != nil {
		return err
	}

	filter, args := innodbIndexStatsFilter(s.Databases)
	rows, err := db.QueryContext(ctx, strings.Replace(innodbIndexStatsQuery, "%s", filter, 1), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema, table, index, statName, description string
			statValue                                   uint64
			lastUpdate                                  float64
		)
		if err := rows.Scan(&schema, &table, &index, &statName, &statValue, &description, &lastUpdate); err != nil {
			return err
		}

		switch {
		case statName == "size":
			ch <- prometheus.MustNewConstMetric(
				innodbIndexStatsSizeDesc, prometheus.GaugeValue, float64(statValue)*pageSize,
				schema, table, index,
			)
			// Every index has a size row, so use it to report when the
			// statistics of the index were last updated.
			ch <- prometheus.MustNewConstMetric(
				innodbIndexStatsLastUpdateDesc, prometheus.GaugeValue, lastUpdate,
				schema, table, index,
			)
		case statName == "n_leaf_pages":
			ch <- prometheus.MustNewConstMetric(
				innodbIndexStatsLeafSizeDesc, prometheus.GaugeValue, float64(statValue)*pageSize,
				schema, table, index,
			)
		case strings.HasPrefix(statName, "n_diff_pfx"):
			// The description lists the leading columns the estimate covers,
			// e.g. "customer_id,created_at".
			ch <- prometheus.MustNewConstMetric(
				innodbIndexStatsDistinctValuesDesc, prometheus.GaugeValue, float64(statValue),
				schema, table, index, description,
			)
		}
	}

	return rows.Err()
}

// innodbIndexStatsFilter returns the WHERE clause and its arguments for the
// configured databases: "*" means every database except the system schemas,
// anything else is a comma separated list of database names.
func innodbIndexStatsFilter(databases string) (string, []any) {
	if databases == "*" {
		return innodbIndexStatsSystemSchemasFilter, nil
	}

	var (
		placeholders []string
		args         []any
	)
	for _, database := range strings.Split(databases, ",") {
		database = strings.TrimSpace(database)
		if database == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, database)
	}
	if len(placeholders) == 0 {
		return innodbIndexStatsSystemSchemasFilter, nil
	}

	return "database_name IN (" + strings.Join(placeholders, ", ") + ")", args
}

// check interface
var _ Scraper = ScrapeInnodbIndexStats{}
