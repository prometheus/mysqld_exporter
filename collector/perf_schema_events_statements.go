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

// Scrape `performance_schema.events_statements_summary_by_digest`.

package collector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const perfEventsStatementsQuery = `
	SELECT
	    ifnull(SCHEMA_NAME, 'NONE') as SCHEMA_NAME,
	    DIGEST,
	    LEFT(DIGEST_TEXT, %d) as DIGEST_TEXT,
	    COUNT_STAR,
	    SUM_TIMER_WAIT,
	    SUM_ERRORS,
	    SUM_WARNINGS,
	    SUM_ROWS_AFFECTED,
	    SUM_ROWS_SENT,
	    SUM_ROWS_EXAMINED,
	    SUM_CREATED_TMP_DISK_TABLES,
	    SUM_CREATED_TMP_TABLES,
	    SUM_SORT_MERGE_PASSES,
	    SUM_SORT_ROWS,
	    SUM_NO_INDEX_USED
	  FROM (
	    SELECT *
	    FROM performance_schema.events_statements_summary_by_digest
	    WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema')
	      AND LAST_SEEN > DATE_SUB(NOW(), INTERVAL %d SECOND)
	    ORDER BY LAST_SEEN DESC
	  )Q
	  GROUP BY
	    Q.SCHEMA_NAME,
	    Q.DIGEST,
	    Q.DIGEST_TEXT,
	    Q.COUNT_STAR,
	    Q.SUM_TIMER_WAIT,
	    Q.SUM_ERRORS,
	    Q.SUM_WARNINGS,
	    Q.SUM_ROWS_AFFECTED,
	    Q.SUM_ROWS_SENT,
	    Q.SUM_ROWS_EXAMINED,
	    Q.SUM_CREATED_TMP_DISK_TABLES,
	    Q.SUM_CREATED_TMP_TABLES,
	    Q.SUM_SORT_MERGE_PASSES,
	    Q.SUM_SORT_ROWS,
	    Q.SUM_NO_INDEX_USED
	  ORDER BY SUM_TIMER_WAIT DESC
	  LIMIT %d
	`

// Tunable flags.
var (
	perfEventsStatementsLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.limit",
		"Limit the number of events statements digests by response time",
	).Default("250").Int()
	perfEventsStatementsTimeLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.timelimit",
		"Limit how old the 'last_seen' events statements can be, in seconds",
	).Default("86400").Int()
	perfEventsStatementsDigestTextLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.digest_text_limit",
		"Maximum length of the normalized statement text",
	).Default("120").Int()
)

// ScrapePerfEventsStatements collects from `performance_schema.events_statements_summary_by_digest`.
type ScrapePerfEventsStatements struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfEventsStatements) Name() string {
	return "perf_schema.eventsstatements"
}

// Help describes the role of the Scraper.
func (ScrapePerfEventsStatements) Help() string {
	return "Collect metrics from performance_schema.events_statements_summary_by_digest"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfEventsStatements) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfEventsStatements) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, constLabels prometheus.Labels) error {
	perfQuery := fmt.Sprintf(
		perfEventsStatementsQuery,
		*perfEventsStatementsDigestTextLimit,
		*perfEventsStatementsTimeLimit,
		*perfEventsStatementsLimit,
	)
	// Timers here are returned in picoseconds.
	perfSchemaEventsStatementsRows, err := db.QueryContext(ctx, perfQuery)
	if err != nil {
		return err
	}
	defer perfSchemaEventsStatementsRows.Close()

	var (
		schemaName, digest, digestText       string
		count, queryTime, errors, warnings   uint64
		rowsAffected, rowsSent, rowsExamined uint64
		tmpTables, tmpDiskTables             uint64
		sortMergePasses, sortRows            uint64
		noIndexUsed                          uint64
	)
	for perfSchemaEventsStatementsRows.Next() {
		if err := perfSchemaEventsStatementsRows.Scan(
			&schemaName, &digest, &digestText, &count, &queryTime, &errors, &warnings, &rowsAffected, &rowsSent, &rowsExamined, &tmpTables, &tmpDiskTables, &sortMergePasses, &sortRows, &noIndexUsed,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_total", "The total count of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(count),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_seconds_total", "The total time of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(queryTime)/picoSeconds,
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_errors_total", "The errors of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(errors),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_warnings_total", "The warnings of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(warnings),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_rows_affected_total", "The total rows affected of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(rowsAffected),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_rows_sent_total", "The total rows sent of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(rowsSent),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_rows_examined_total", "The total rows examined of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(rowsExamined),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_tmp_tables_total", "The total tmp tables of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(tmpTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_tmp_disk_tables_total", "The total tmp disk tables of events statements by digest.", constLabels, []string{"schema", "digest", "digest_text"}), prometheus.CounterValue, float64(tmpDiskTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_sort_merge_passes_total", "The total number of merge passes by the sort algorithm performed by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(sortMergePasses),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_sort_rows_total", "The total number of sorted rows by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(sortRows),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			newDescLabels(performanceSchema, "events_statements_no_index_used_total", "The total number of statements that used full table scans by digest.", constLabels, []string{"schema", "digest", "digest_text"}),
			prometheus.CounterValue, float64(noIndexUsed),
			schemaName, digest, digestText,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfEventsStatements{}
