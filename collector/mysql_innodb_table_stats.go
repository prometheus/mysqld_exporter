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

// Scrape `mysql.innodb_index_stats`.

package collector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const mysqlTableStatsQuery = `
		  SELECT
		    database_name,
		    table_name,
		    n_rows,
            clustered_index_size,
            sum_of_other_index_sizes
		  FROM mysql.innodb_table_stats
		`

var (
	tableStatLabelNames = []string{"database_name", "table_name"}
)

// Metric descriptors.
var (
	nRowsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "innodb_table_stats_n_rows"),
		"Stores data related to particular InnoDB Persistent Statistics.",
		tableStatLabelNames, nil)
	clusteredIndexSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "innodb_table_stats_clustered_index_size"),
		"Stores data related to particular InnoDB Persistent Statistics.",
		tableStatLabelNames, nil)
	sumOfOtherIndexSizesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "innodb_table_stats_sum_of_other_index_sizes"),
		"Stores data related to particular InnoDB Persistent Statistics.",
		tableStatLabelNames, nil)
)

type ScrapeMysqlTableStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeMysqlTableStat) Name() string {
	return "mysql.innodb_table_stats"
}

// Help describes the role of the Scraper.
func (ScrapeMysqlTableStat) Help() string {
	return "Collect data from mysql.innodb_table_stats"
}

// Version of MySQL from which scraper is available.
func (ScrapeMysqlTableStat) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeMysqlTableStat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var (
		tableStatRows *sql.Rows
		err           error
	)
	tableStatsQuery := fmt.Sprint(mysqlTableStatsQuery)
	tableStatRows, err = db.QueryContext(ctx, tableStatsQuery)
	if err != nil {
		return err
	}
	defer tableStatRows.Close()

	var (
		database_name            string
		table_name               string
		n_rows                   uint32
		clustered_index_size     uint32
		sum_of_other_index_sizes uint32
	)

	for tableStatRows.Next() {
		err = tableStatRows.Scan(
			&database_name,
			&table_name,
			&n_rows,
			&clustered_index_size,
			&sum_of_other_index_sizes,
		)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(nRowsDesc, prometheus.GaugeValue, float64(n_rows), database_name, table_name)
		ch <- prometheus.MustNewConstMetric(clusteredIndexSizeDesc, prometheus.GaugeValue, float64(clustered_index_size), database_name, table_name)
		ch <- prometheus.MustNewConstMetric(sumOfOtherIndexSizesDesc, prometheus.GaugeValue, float64(sum_of_other_index_sizes), database_name, table_name)
	}

	return nil
}

var _ Scraper = ScrapeMysqlTableStat{}
