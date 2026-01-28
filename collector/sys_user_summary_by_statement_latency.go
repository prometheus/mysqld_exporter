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
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

type ScrapeSysUserSummaryByStatementLatency struct{}

func (ScrapeSysUserSummaryByStatementLatency) Name() string {
	return "sys.user_summary_by_statement_latency"
}
func (ScrapeSysUserSummaryByStatementLatency) Help() string {
	return "Collect metrics from sys.x$user_summary_by_statement_latency."
}
func (ScrapeSysUserSummaryByStatementLatency) Version() float64 { return 5.7 }

// Metric name stem to match sys_user_summary.go style.
const userSummaryByStmtLatencyStem = "user_summary_by_statement_latency"

// Descriptors (namespace=sys schema; names include the stem above).
var (
	sysUSSBLStatementsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_total"),
		"The total number of statements for the user.",
		[]string{"user"}, nil,
	)
	sysUSSBLTotalLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_latency"),
		"The total wait time of timed statements for the user (seconds).",
		[]string{"user"}, nil,
	)
	sysUSSBLMaxLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_max_latency"),
		"The maximum single-statement latency for the user (seconds).",
		[]string{"user"}, nil,
	)
	sysUSSBLLockLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_lock_latency"),
		"The total time spent waiting for locks for the user (seconds).",
		[]string{"user"}, nil,
	)
	sysUSSBLCpuLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_cpu_latency"),
		"The total CPU time spent by statements for the user (seconds).",
		[]string{"user"}, nil,
	)
	sysUSSBLRowsSent = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_rows_sent_total"),
		"The total number of rows sent by statements for the user.",
		[]string{"user"}, nil,
	)
	sysUSSBLRowsExamined = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_rows_examined_total"),
		"The total number of rows examined by statements for the user.",
		[]string{"user"}, nil,
	)
	sysUSSBLRowsAffected = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_rows_affected_total"),
		"The total number of rows affected by statements for the user.",
		[]string{"user"}, nil,
	)
	sysUSSBLFullScans = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtLatencyStem+"_full_scans_total"),
		"The total number of full table scans by statements for the user.",
		[]string{"user"}, nil,
	)
)

func (ScrapeSysUserSummaryByStatementLatency) Scrape(
	ctx context.Context,
	inst *instance,
	ch chan<- prometheus.Metric,
	_ *slog.Logger,
) error {
	const q = `
SELECT
  user,
  total,
  total_latency,
  max_latency,
  lock_latency,
  cpu_latency,
  rows_sent,
  rows_examined,
  rows_affected,
  full_scans
FROM sys.x$user_summary_by_statement_latency`

	rows, err := inst.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			user                              string
			total                             uint64
			totalPs, maxPs, lockPs, cpuPs     uint64
			rowsSent, rowsExam, rowsAff, fscs uint64
		)
		if err := rows.Scan(&user, &total, &totalPs, &maxPs, &lockPs, &cpuPs, &rowsSent, &rowsExam, &rowsAff, &fscs); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(sysUSSBLStatementsTotal, prometheus.GaugeValue, float64(total), user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLTotalLatency, prometheus.GaugeValue, float64(totalPs)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLMaxLatency, prometheus.GaugeValue, float64(maxPs)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLLockLatency, prometheus.GaugeValue, float64(lockPs)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLCpuLatency, prometheus.GaugeValue, float64(cpuPs)/picoSeconds, user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLRowsSent, prometheus.GaugeValue, float64(rowsSent), user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLRowsExamined, prometheus.GaugeValue, float64(rowsExam), user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLRowsAffected, prometheus.GaugeValue, float64(rowsAff), user)
		ch <- prometheus.MustNewConstMetric(sysUSSBLFullScans, prometheus.GaugeValue, float64(fscs), user)
	}
	return rows.Err()
}
