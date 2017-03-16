// Scrape `performance_schema.file_summary_by_event_name`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"

	"strings"
)

const perfFileInstancesQuery = `
	SELECT
	    FILE_NAME,
	    COUNT_READ, COUNT_WRITE,
	    SUM_NUMBER_OF_BYTES_READ, SUM_NUMBER_OF_BYTES_WRITE
	  FROM performance_schema.file_summary_by_instance
	     where LOCATE(?,FILE_NAME)>0
	`

// Metric descriptors.
var (
	performanceSchemaFileInstancesBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_instances_bytes"),
		"The number of bytes read or write by the file.",
		[]string{"file", "mode"}, nil,
	)
	performanceSchemaFileInstancesCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_instances_count"),
		"The number of operations by the file.",
		[]string{"file", "mode"}, nil,
	)
)

// ScrapePerfFileEvents collects from `performance_schema.file_summary_by_event_name`.
func ScrapePerfFileInstances(db *sql.DB, ch chan<- prometheus.Metric, filter *string) error {
	// Timers here are returned in picoseconds.
	perfSchemaFileInstancesRows, err := db.Query(perfFileInstancesQuery, *filter)
	if err != nil {
		return err
	}
	defer perfSchemaFileInstancesRows.Close()

	var (
		fileName                      string
		countRead, countWrite         uint64
		sumBytesRead, sumBytesWritten uint64
	)
	for perfSchemaFileInstancesRows.Next() {
		if err := perfSchemaFileInstancesRows.Scan(
			&fileName,
			&countRead, &countWrite,
			&sumBytesRead, &sumBytesWritten,
		); err != nil {
			return err
		}
		if len(*filter) > 0 {
			pos:=strings.LastIndex(fileName,*filter)
			if pos>-1 {
				fileName=fileName[pos+len(*filter):]
			}
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesCountDesc, prometheus.CounterValue, float64(countRead),
			fileName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesCountDesc , prometheus.CounterValue, float64(countWrite),
			fileName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesBytesDesc, prometheus.CounterValue, float64(sumBytesRead),
			fileName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileInstancesBytesDesc, prometheus.CounterValue, float64(sumBytesWritten),
			fileName, "write",
		)

	}
	return nil
}
