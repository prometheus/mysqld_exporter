// Scrape `performance_schema.table_io_waits_summary_by_table`.

package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

const perfTableIOWaitsQuery = `
	SELECT
	    OBJECT_SCHEMA, OBJECT_NAME,
	    COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE,
	    SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
	  FROM performance_schema.table_io_waits_summary_by_table
	  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
	`

// Metric descriptors.
var (
	performanceSchemaTableWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "table_io_waits_total"),
		"The total number of table I/O wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaTableWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "table_io_waits_seconds_total"),
		"The total time of table I/O wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
)

// ScrapePerfTableIOWaits collects from `performance_schema.table_io_waits_summary_by_table`.
type ScrapePerfTableIOWaits struct{}

// Name of the Scraper.
func (ScrapePerfTableIOWaits) Name() string {
	return "perf_schema.tableiowaits"
}

// Help returns additional information about Scraper.
func (ScrapePerfTableIOWaits) Help() string {
	return "Collect metrics from performance_schema.table_io_waits_summary_by_table"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfTableIOWaits) Version() float64 {
	return 5.6
}

// Scrape collects data.
func (ScrapePerfTableIOWaits) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	perfSchemaTableWaitsRows, err := db.QueryContext(ctx, perfTableIOWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableWaitsRows.Close()

	var (
		objectSchema, objectName                          string
		countFetch, countInsert, countUpdate, countDelete uint64
		timeFetch, timeInsert, timeUpdate, timeDelete     uint64
	)

	for perfSchemaTableWaitsRows.Next() {
		if err := perfSchemaTableWaitsRows.Scan(
			&objectSchema, &objectName, &countFetch, &countInsert, &countUpdate, &countDelete,
			&timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countFetch),
			objectSchema, objectName, "fetch",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countInsert),
			objectSchema, objectName, "insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countUpdate),
			objectSchema, objectName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countDelete),
			objectSchema, objectName, "delete",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeFetch)/picoSeconds,
			objectSchema, objectName, "fetch",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeInsert)/picoSeconds,
			objectSchema, objectName, "insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeUpdate)/picoSeconds,
			objectSchema, objectName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeDelete)/picoSeconds,
			objectSchema, objectName, "delete",
		)
	}
	return nil
}
