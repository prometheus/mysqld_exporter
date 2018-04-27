// Scrape `SHOW GLOBAL STATUS`.

package collector

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Scrape query.
	globalStatusQuery = `SHOW GLOBAL STATUS`
	// Subsystem.
	globalStatus = "global_status"
)

// Regexp to match various groups of status vars.
var globalStatusRE = regexp.MustCompile(`^(com|handler|connection_errors|innodb_buffer_pool_pages|innodb_rows|performance_schema)_(.*)$`)

// Metric descriptors.
var (
	globalCommandsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "commands_total"),
		"Total number of executed MySQL commands.",
		[]string{"command"}, nil,
	)
	globalHandlerDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "handlers_total"),
		"Total number of executed MySQL handlers.",
		[]string{"handler"}, nil,
	)
	globalConnectionErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "connection_errors_total"),
		"Total number of MySQL connection errors.",
		[]string{"error"}, nil,
	)
	globalBufferPoolPagesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "buffer_pool_pages"),
		"Innodb buffer pool pages by state.",
		[]string{"state"}, nil,
	)
	globalBufferPoolPageChangesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "buffer_pool_page_changes_total"),
		"Innodb buffer pool page state changes.",
		[]string{"operation"}, nil,
	)
	globalInnoDBRowOpsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "innodb_row_ops_total"),
		"Total number of MySQL InnoDB row operations.",
		[]string{"operation"}, nil,
	)
	globalPerformanceSchemaLostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "performance_schema_lost_total"),
		"Total number of MySQL instrumentations that could not be loaded or created due to memory constraints.",
		[]string{"instrumentation"}, nil,
	)
)

// ScrapeGlobalStatus collects from `SHOW GLOBAL STATUS`.
type ScrapeGlobalStatus struct{}

// Name of the Scraper. Should be unique.
func (ScrapeGlobalStatus) Name() string {
	return globalStatus
}

// Help describes the role of the Scraper.
func (ScrapeGlobalStatus) Help() string {
	return "Collect from SHOW GLOBAL STATUS"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeGlobalStatus) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	globalStatusRows, err := db.Query(globalStatusQuery)
	if err != nil {
		return err
	}
	defer globalStatusRows.Close()

	var key string
	var val sql.RawBytes
	var textItems = map[string]string{
		"wsrep_local_state_uuid":   "",
		"wsrep_cluster_state_uuid": "",
		"wsrep_provider_version":   "",
	}

	for globalStatusRows.Next() {
		if err := globalStatusRows.Scan(&key, &val); err != nil {
			return err
		}
		if floatVal, ok := parseStatus(val); ok { // Unparsable values are silently skipped.
			key = strings.ToLower(key)
			match := globalStatusRE.FindStringSubmatch(key)
			if match == nil {
				ch <- prometheus.MustNewConstMetric(
					newDesc(globalStatus, key, "Generic metric from SHOW GLOBAL STATUS."),
					prometheus.UntypedValue,
					floatVal,
				)
				continue
			}
			switch match[1] {
			case "com":
				ch <- prometheus.MustNewConstMetric(
					globalCommandsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "handler":
				ch <- prometheus.MustNewConstMetric(
					globalHandlerDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "connection_errors":
				ch <- prometheus.MustNewConstMetric(
					globalConnectionErrorsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "innodb_buffer_pool_pages":
				switch match[2] {
				case "data", "dirty", "free", "misc":
					ch <- prometheus.MustNewConstMetric(
						globalBufferPoolPagesDesc, prometheus.GaugeValue, floatVal, match[2],
					)
				default:
					ch <- prometheus.MustNewConstMetric(
						globalBufferPoolPageChangesDesc, prometheus.CounterValue, floatVal, match[2],
					)
				}
			case "innodb_rows":
				ch <- prometheus.MustNewConstMetric(
					globalInnoDBRowOpsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "performance_schema":
				ch <- prometheus.MustNewConstMetric(
					globalPerformanceSchemaLostDesc, prometheus.CounterValue, floatVal, match[2],
				)
			}
		} else if _, ok := textItems[key]; ok {
			textItems[key] = string(val)
		}
	}

	// mysql_galera_variables_info metric.
	if textItems["wsrep_local_state_uuid"] != "" {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "galera", "status_info"), "PXC/Galera status information.",
				[]string{"wsrep_local_state_uuid", "wsrep_cluster_state_uuid", "wsrep_provider_version"}, nil),
			prometheus.GaugeValue, 1, textItems["wsrep_local_state_uuid"], textItems["wsrep_cluster_state_uuid"], textItems["wsrep_provider_version"],
		)
	}

	return nil
}
