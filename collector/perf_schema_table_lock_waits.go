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

// Scrape `performance_schema.table_lock_waits_summary_by_table`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfTableLockWaitsQuery = `
	SELECT
	    OBJECT_SCHEMA,
	    OBJECT_NAME,
	    COUNT_READ_NORMAL,
	    COUNT_READ_WITH_SHARED_LOCKS,
	    COUNT_READ_HIGH_PRIORITY,
	    COUNT_READ_NO_INSERT,
	    COUNT_READ_EXTERNAL,
	    COUNT_WRITE_ALLOW_WRITE,
	    COUNT_WRITE_CONCURRENT_INSERT,
	    COUNT_WRITE_LOW_PRIORITY,
	    COUNT_WRITE_NORMAL,
	    COUNT_WRITE_EXTERNAL,
	    SUM_TIMER_READ_NORMAL,
	    SUM_TIMER_READ_WITH_SHARED_LOCKS,
	    SUM_TIMER_READ_HIGH_PRIORITY,
	    SUM_TIMER_READ_NO_INSERT,
	    SUM_TIMER_READ_EXTERNAL,
	    SUM_TIMER_WRITE_ALLOW_WRITE,
	    SUM_TIMER_WRITE_CONCURRENT_INSERT,
	    SUM_TIMER_WRITE_LOW_PRIORITY,
	    SUM_TIMER_WRITE_NORMAL,
	    SUM_TIMER_WRITE_EXTERNAL
	  FROM performance_schema.table_lock_waits_summary_by_table
	  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema')
	`

// Metric descriptors.
var (
	performanceSchemaSQLTableLockWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "sql_lock_waits_total"),
		"The total number of SQL lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaExternalTableLockWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "external_lock_waits_total"),
		"The total number of external lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaSQLTableLockWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "sql_lock_waits_seconds_total"),
		"The total time of SQL lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaExternalTableLockWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "external_lock_waits_seconds_total"),
		"The total time of external lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
)

// ScrapePerfTableLockWaits collects from `performance_schema.table_lock_waits_summary_by_table`.
type ScrapePerfTableLockWaits struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfTableLockWaits) Name() string {
	return "perf_schema.tablelocks"
}

// Help describes the role of the Scraper.
func (ScrapePerfTableLockWaits) Help() string {
	return "Collect metrics from performance_schema.table_lock_waits_summary_by_table"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfTableLockWaits) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfTableLockWaits) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	perfSchemaTableLockWaitsRows, err := db.QueryContext(ctx, perfTableLockWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableLockWaitsRows.Close()

	var (
		objectSchema               string
		objectName                 string
		countReadNormal            uint64
		countReadWithSharedLocks   uint64
		countReadHighPriority      uint64
		countReadNoInsert          uint64
		countReadExternal          uint64
		countWriteAllowWrite       uint64
		countWriteConcurrentInsert uint64
		countWriteLowPriority      uint64
		countWriteNormal           uint64
		countWriteExternal         uint64
		timeReadNormal             uint64
		timeReadWithSharedLocks    uint64
		timeReadHighPriority       uint64
		timeReadNoInsert           uint64
		timeReadExternal           uint64
		timeWriteAllowWrite        uint64
		timeWriteConcurrentInsert  uint64
		timeWriteLowPriority       uint64
		timeWriteNormal            uint64
		timeWriteExternal          uint64
	)

	for perfSchemaTableLockWaitsRows.Next() {
		if err := perfSchemaTableLockWaitsRows.Scan(
			&objectSchema,
			&objectName,
			&countReadNormal,
			&countReadWithSharedLocks,
			&countReadHighPriority,
			&countReadNoInsert,
			&countReadExternal,
			&countWriteAllowWrite,
			&countWriteConcurrentInsert,
			&countWriteLowPriority,
			&countWriteNormal,
			&countWriteExternal,
			&timeReadNormal,
			&timeReadWithSharedLocks,
			&timeReadHighPriority,
			&timeReadNoInsert,
			&timeReadExternal,
			&timeWriteAllowWrite,
			&timeWriteConcurrentInsert,
			&timeWriteLowPriority,
			&timeWriteNormal,
			&timeWriteExternal,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadNormal),
			objectSchema, objectName, "read_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadWithSharedLocks),
			objectSchema, objectName, "read_with_shared_locks",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadHighPriority),
			objectSchema, objectName, "read_high_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadNoInsert),
			objectSchema, objectName, "read_no_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteNormal),
			objectSchema, objectName, "write_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteAllowWrite),
			objectSchema, objectName, "write_allow_write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteConcurrentInsert),
			objectSchema, objectName, "write_concurrent_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteLowPriority),
			objectSchema, objectName, "write_low_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsDesc, prometheus.CounterValue, float64(countReadExternal),
			objectSchema, objectName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteExternal),
			objectSchema, objectName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadNormal)/picoSeconds,
			objectSchema, objectName, "read_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadWithSharedLocks)/picoSeconds,
			objectSchema, objectName, "read_with_shared_locks",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadHighPriority)/picoSeconds,
			objectSchema, objectName, "read_high_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadNoInsert)/picoSeconds,
			objectSchema, objectName, "read_no_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteNormal)/picoSeconds,
			objectSchema, objectName, "write_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteAllowWrite)/picoSeconds,
			objectSchema, objectName, "write_allow_write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteConcurrentInsert)/picoSeconds,
			objectSchema, objectName, "write_concurrent_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteLowPriority)/picoSeconds,
			objectSchema, objectName, "write_low_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadExternal)/picoSeconds,
			objectSchema, objectName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteExternal)/picoSeconds,
			objectSchema, objectName, "write",
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfTableLockWaits{}
