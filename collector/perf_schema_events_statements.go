// Scrape `performance_schema.events_statements_summary_by_digest`.

package collector

import (
	"context"
	"database/sql"
	"flag"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
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

// Tuning flags.
var (
	perfEventsStatementsLimit = flag.Int(
		"collect.perf_schema.eventsstatements.limit", 250,
		"Limit the number of events statements digests by response time",
	)
	perfEventsStatementsTimeLimit = flag.Int(
		"collect.perf_schema.eventsstatements.timelimit", 86400,
		"Limit how old the 'last_seen' events statements can be, in seconds",
	)
	perfEventsStatementsDigestTextLimit = flag.Int(
		"collect.perf_schema.eventsstatements.digest_text_limit", 120,
		"Maximum length of the normalized statement text",
	)
)

// Metric descriptors.
var (
	performanceSchemaEventsStatementsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_total"),
		"The total count of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_seconds_total"),
		"The total time of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_errors_total"),
		"The errors of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsWarningsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_warnings_total"),
		"The warnings of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsAffectedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_affected_total"),
		"The total rows affected of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_sent_total"),
		"The total rows sent of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsExaminedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_examined_total"),
		"The total rows examined of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTmpTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_tables_total"),
		"The total tmp tables of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTmpDiskTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_disk_tables_total"),
		"The total tmp disk tables of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsSortMergePassesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_sort_merge_passes_total"),
		"The total number of merge passes by the sort algorithm performed by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsSortRowsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_sort_rows_total"),
		"The total number of sorted rows by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsNoIndexUsedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_no_index_used_total"),
		"The total number of statements that used full table scans by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
)

// ScrapePerfEventsStatements collects from `performance_schema.events_statements_summary_by_digest`.
type ScrapePerfEventsStatements struct{}

// Name of the Scraper.
func (ScrapePerfEventsStatements) Name() string {
	return "perf_schema.eventsstatements"
}

// Help returns additional information about Scraper.
func (ScrapePerfEventsStatements) Help() string {
	return "Collect metrics from performance_schema.events_statements_summary_by_digest"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfEventsStatements) Version() float64 {
	return 5.6
}

// Scrape collects data.
func (ScrapePerfEventsStatements) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
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
			performanceSchemaEventsStatementsDesc, prometheus.CounterValue, float64(count),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTimeDesc, prometheus.CounterValue, float64(queryTime)/picoSeconds,
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsErrorsDesc, prometheus.CounterValue, float64(errors),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsWarningsDesc, prometheus.CounterValue, float64(warnings),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsAffectedDesc, prometheus.CounterValue, float64(rowsAffected),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsSentDesc, prometheus.CounterValue, float64(rowsSent),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsExaminedDesc, prometheus.CounterValue, float64(rowsExamined),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpTablesDesc, prometheus.CounterValue, float64(tmpTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpDiskTablesDesc, prometheus.CounterValue, float64(tmpDiskTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsSortMergePassesDesc, prometheus.CounterValue, float64(sortMergePasses),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsSortRowsDesc, prometheus.CounterValue, float64(sortRows),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsNoIndexUsedDesc, prometheus.CounterValue, float64(noIndexUsed),
			schemaName, digest, digestText,
		)
	}
	return nil
}
