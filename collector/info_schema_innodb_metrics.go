// Scrape `information_schema.innodb_metrics`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaInnodbMetricsQuery = `
		SELECT
		  name, subsystem, type, comment,
		  count
		  FROM information_schema.innodb_metrics
		  WHERE status = 'enabled'
		`

// ScrapeInnodbMetrics collects from `information_schema.innodb_metrics`.
func ScrapeInnodbMetrics(db *sql.DB, ch chan<- prometheus.Metric) error {
	innodbMetricsRows, err := db.Query(infoSchemaInnodbMetricsQuery)
	if err != nil {
		return err
	}
	defer innodbMetricsRows.Close()

	var (
		name, subsystem, metricType, comment string
		value                                int64
	)

	for innodbMetricsRows.Next() {
		if err := innodbMetricsRows.Scan(
			&name, &subsystem, &metricType, &comment, &value,
		); err != nil {
			return err
		}
		metricName := "innodb_metrics_" + subsystem + "_" + name
		// MySQL returns counters named two different ways. "counter" and "status_counter"
		// value >= 0 is necessary due to upstream bugs: http://bugs.mysql.com/bug.php?id=75966
		if (metricType == "counter" || metricType == "status_counter") && value >= 0 {
			description := prometheus.NewDesc(
				prometheus.BuildFQName(namespace, informationSchema, metricName+"_total"),
				comment, nil, nil,
			)
			ch <- prometheus.MustNewConstMetric(
				description,
				prometheus.CounterValue,
				float64(value),
			)
		} else {
			description := prometheus.NewDesc(
				prometheus.BuildFQName(namespace, informationSchema, metricName),
				comment, nil, nil,
			)
			ch <- prometheus.MustNewConstMetric(
				description,
				prometheus.GaugeValue,
				float64(value),
			)
		}
	}
	return nil
}
