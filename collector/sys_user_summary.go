// Copyright 2022 The Prometheus Authors
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

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const sysUserSummaryQuery = `
	SELECT
		user,
		statements,
		statement_latency,
		table_scans,
		file_ios,
		file_io_latency,
		current_connections,
		total_connections,
		unique_hosts,
		current_memory,
		total_memory_allocated
	FROM
		` + sysSchema + `.x$user_summary
`

var (
	sysUserSummaryStatements = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "statements_total"),
		" The total number of statements for the user",
		[]string{"user"}, nil)
	sysUserSummaryStatementLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "statement_latency"),
		"The total wait time of timed statements for the user",
		[]string{"user"}, nil)
	sysUserSummaryTableScans = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "table_scans_total"),
		"The total number of table scans for the user",
		[]string{"user"}, nil)
	sysUserSummaryFileIOs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "file_ios_total"),
		"The total number of file I/O events for the user",
		[]string{"user"}, nil)
	sysUserSummaryFileIOLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "file_io_seconds_total"),
		"The total wait time of timed file I/O events for the user",
		[]string{"user"}, nil)
	sysUserSummaryCurrentConnections = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "current_connections"),
		"The current number of connections for the user",
		[]string{"user"}, nil)
	sysUserSummaryTotalConnections = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "connections_total"),
		"The total number of connections for the user",
		[]string{"user"}, nil)
	sysUserSummaryUniqueHosts = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "unique_hosts_total"),
		"The number of distinct hosts from which connections for the user have originated",
		[]string{"user"}, nil)
	sysUserSummaryCurrentMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "current_memory_bytes"),
		"The current amount of allocated memory for the user",
		[]string{"user"}, nil)
	sysUserSummaryTotalMemoryAllocated = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, "memory_allocated_bytes_total"),
		"The total amount of allocated memory for the user",
		[]string{"user"}, nil)
)

type ScrapeSysUserSummary struct{}

// Name of the Scraper. Should be unique.
func (ScrapeSysUserSummary) Name() string {
	return sysSchema + ".user_summary"
}

// Help describes the role of the Scraper.
func (ScrapeSysUserSummary) Help() string {
	return "Collect per user metrics from sys.x$user_summary. See https://dev.mysql.com/doc/refman/5.7/en/sys-user-summary.html for details"
}

// Version of MySQL from which scraper is available.
func (ScrapeSysUserSummary) Version() float64 {
	return 5.7
}

// Scrape the information from sys.user_summary, creating a metric for each value of each row, labeled with the user
func (ScrapeSysUserSummary) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {

	userSummaryRows, err := db.QueryContext(ctx, sysUserSummaryQuery)
	if err != nil {
		return err
	}
	defer userSummaryRows.Close()

	var (
		user                   string
		statements             uint64
		statement_latency      float64
		table_scans            uint64
		file_ios               uint64
		file_io_latency        float64
		current_connections    uint64
		total_connections      uint64
		unique_hosts           uint64
		current_memory         uint64
		total_memory_allocated uint64
	)

	for userSummaryRows.Next() {
		err = userSummaryRows.Scan(
			&user,
			&statements,
			&statement_latency,
			&table_scans,
			&file_ios,
			&file_io_latency,
			&current_connections,
			&total_connections,
			&unique_hosts,
			&current_memory,
			&total_memory_allocated,
		)
		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(sysUserSummaryStatements, prometheus.CounterValue, float64(statements), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryStatementLatency, prometheus.CounterValue, float64(statement_latency)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryTableScans, prometheus.CounterValue, float64(table_scans), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryFileIOs, prometheus.CounterValue, float64(file_ios), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryFileIOLatency, prometheus.CounterValue, float64(file_io_latency)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryCurrentConnections, prometheus.GaugeValue, float64(current_connections), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryTotalConnections, prometheus.CounterValue, float64(total_connections), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryUniqueHosts, prometheus.CounterValue, float64(unique_hosts), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryCurrentMemory, prometheus.GaugeValue, float64(current_memory), user)
		ch <- prometheus.MustNewConstMetric(sysUserSummaryTotalMemoryAllocated, prometheus.CounterValue, float64(total_memory_allocated), user)

	}
	return nil
}

var _ Scraper = ScrapeSysUserSummary{}
