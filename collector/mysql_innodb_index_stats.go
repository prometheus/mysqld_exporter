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

const mysqlIndexStatsQuery = `
		  SELECT
		    database_name,
		    table_name,
		    index_name,
            stat_name,
            stat_value
		  FROM mysql.innodb_index_stats
		`

var (
	indexStatLabelNames = []string{"database_name", "table_name", "index_name", "stat_name"}
)

// Metric descriptors.
var (
	indexStatsValueDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "innodb_index_stats_stat_value"),
		"Size of the InnoDB index in bytes.",
		indexStatLabelNames, nil)
)

type ScrapeMysqlIndexStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeMysqlIndexStat) Name() string {
	return "mysql.innodb_index_stats"
}

// Help describes the role of the Scraper.
func (ScrapeMysqlIndexStat) Help() string {
	return "Collect data from mysql.innodb_index_stats"
}

// Version of MySQL from which scraper is available.
func (ScrapeMysqlIndexStat) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeMysqlIndexStat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var (
		indexStatRows *sql.Rows
		err           error
	)
	indexStatsQuery := fmt.Sprint(mysqlIndexStatsQuery)
	indexStatRows, err = db.QueryContext(ctx, indexStatsQuery)
	if err != nil {
		return err
	}
	defer indexStatRows.Close()

	var (
		database_name string
		table_name    string
		index_name    string
		stat_name     string
		stat_value    uint32
	)

	for indexStatRows.Next() {
		err = indexStatRows.Scan(
			&database_name,
			&table_name,
			&index_name,
			&stat_name,
			&stat_value,
		)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(indexStatsValueDesc, prometheus.GaugeValue, float64(stat_value), database_name, table_name, index_name, stat_name)
	}

	return nil
}

var _ Scraper = ScrapeMysqlIndexStat{}
