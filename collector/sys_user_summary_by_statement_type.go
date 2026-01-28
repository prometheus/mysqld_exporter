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

type ScrapeSysUserSummaryByStatementType struct{}

func (ScrapeSysUserSummaryByStatementType) Name() string { return "sys.user_summary_by_statement_type" }
func (ScrapeSysUserSummaryByStatementType) Help() string {
	return "Collect metrics from sys.x$user_summary_by_statement_type."
}
func (ScrapeSysUserSummaryByStatementType) Version() float64 { return 5.7 }

// Metric name stem to match sys_user_summary.go style.
const userSummaryByStmtTypeStem = "user_summary_by_statement_type"

// Descriptors.
var (
	sysUSSTStatementsTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_total"),
		"The total number of occurrences of the statement type for the user.",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTTotalLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_latency"),
		"The total wait time of timed occurrences for the user and statement type (seconds).",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTMaxLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_max_latency"),
		"The maximum single-statement latency for the user and statement type (seconds).",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTLockLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_lock_latency"),
		"The total time spent waiting for locks for the user and statement type (seconds).",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTCpuLatency = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_cpu_latency"),
		"The total CPU time for the user and statement type (seconds).",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTRowsSent = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_rows_sent_total"),
		"The total number of rows sent for the user and statement type.",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTRowsExamined = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_rows_examined_total"),
		"The total number of rows examined for the user and statement type.",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTRowsAffected = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_rows_affected_total"),
		"The total number of rows affected for the user and statement type.",
		[]string{"user", "statement"}, nil,
	)
	sysUSSTFullScans = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, sysSchema, userSummaryByStmtTypeStem+"_full_scans_total"),
		"The total number of full table scans for the user and statement type.",
		[]string{"user", "statement"}, nil,
	)
)

func (ScrapeSysUserSummaryByStatementType) Scrape(
	ctx context.Context,
	inst *instance,
	ch chan<- prometheus.Metric,
	_ *slog.Logger,
) error {
	const q = `
SELECT
  user,
  statement,
  total,
  total_latency,
  max_latency,
  lock_latency,
  cpu_latency,
  rows_sent,
  rows_examined,
  rows_affected,
  full_scans
FROM sys.x$user_summary_by_statement_type`

	rows, err := inst.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			user, stmt                        string
			total                             uint64
			totalPs, maxPs, lockPs, cpuPs     uint64
			rowsSent, rowsExam, rowsAff, fscs uint64
		)
		if err := rows.Scan(&user, &stmt, &total, &totalPs, &maxPs, &lockPs, &cpuPs, &rowsSent, &rowsExam, &rowsAff, &fscs); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(sysUSSTStatementsTotal, prometheus.GaugeValue, float64(total), user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTTotalLatency, prometheus.GaugeValue, float64(totalPs)/picoSeconds, user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTMaxLatency, prometheus.GaugeValue, float64(maxPs)/picoSeconds, user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTLockLatency, prometheus.GaugeValue, float64(lockPs)/picoSeconds, user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTCpuLatency, prometheus.GaugeValue, float64(cpuPs)/picoSeconds, user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTRowsSent, prometheus.GaugeValue, float64(rowsSent), user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTRowsExamined, prometheus.GaugeValue, float64(rowsExam), user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTRowsAffected, prometheus.GaugeValue, float64(rowsAff), user, stmt)
		ch <- prometheus.MustNewConstMetric(sysUSSTFullScans, prometheus.GaugeValue, float64(fscs), user, stmt)
	}
	return rows.Err()
}
