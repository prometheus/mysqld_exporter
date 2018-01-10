// Scrape `performance_schema.file_summary_by_instance`.

package collector

import (
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const perfFileInstancesQuery = `
	SELECT
	    FILE_NAME, EVENT_NAME,
	    COUNT_READ, COUNT_WRITE,
	    SUM_NUMBER_OF_BYTES_READ, SUM_NUMBER_OF_BYTES_WRITE
	  FROM performance_schema.file_summary_by_instance
	     where FILE_NAME REGEXP ?
	`

// Metric descriptors.
var (
	performanceSchemaFileInstancesFilter = kingpin.Flag(
		"collect.perf_schema.file_instances.filter",
		"RegEx file_name filter for performance_schema.file_summary_by_instance",
	).Default(".*").String()

	performanceSchemaFileInstancesRemovePrefix = kingpin.Flag(
		"collect.perf_schema.file_instances.remove_prefix",
		"Remove path prefix in performance_schema.file_summary_by_instance",
	).Default("/var/lib/mysql/").String()

	performanceSchemaFileInstancesBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_instances_bytes"),
		"The number of bytes processed by file read/write operations.",
		[]string{"file_name", "event_name", "mode"}, nil,
	)
	performanceSchemaFileInstancesCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_instances_total"),
		"The total number of file read/write operations.",
		[]string{"file_name", "event_name", "mode"}, nil,
	)
)

// ScrapePerfFileEvents collects from `performance_schema.file_summary_by_event_name`.
func ScrapePerfFileInstances(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaFileInstancesRows, err := db.Query(perfFileInstancesQuery, *performanceSchemaFileInstancesFilter)
	if err != nil {
		return err
	}
	defer perfSchemaFileInstancesRows.Close()

	var (
		fileName, eventName           string
		countRead, countWrite         uint64
		sumBytesRead, sumBytesWritten uint64
	)

	for perfSchemaFileInstancesRows.Next() {
		if err := perfSchemaFileInstancesRows.Scan(
			&fileName, &eventName,
			&countRead, &countWrite,
			&sumBytesRead, &sumBytesWritten,
		); err != nil {
			return err
		}

		fileName = strings.TrimPrefix(fileName, *performanceSchemaFileInstancesRemovePrefix)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesCountDesc, prometheus.CounterValue, float64(countRead),
			fileName, eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesCountDesc, prometheus.CounterValue, float64(countWrite),
			fileName, eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesBytesDesc, prometheus.CounterValue, float64(sumBytesRead),
			fileName, eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesBytesDesc, prometheus.CounterValue, float64(sumBytesWritten),
			fileName, eventName, "write",
		)

	}
	return nil
}
