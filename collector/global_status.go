// Scrape `SHOW GLOBAL STATUS`.

package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Scrape query.
	globalStatusQuery = `SHOW GLOBAL STATUS`
	// Subsystem.
	globalStatus = "global_status"
)

var replLatencyMap = []string{
	"Minimum",
	"Average",
	"Maximum",
	"Standard Deviation",
	"Sample Size",
}

// Regexp to match various groups of status vars.
var globalStatusRE = regexp.MustCompile(`^(com|handler|connection_errors|innodb_buffer_pool_pages|innodb_rows|performance_schema)_(.*)$`)

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

// Name of the Scraper.
func (ScrapeGlobalStatus) Name() string {
	return globalStatus
}

// Help returns additional information about Scraper.
func (ScrapeGlobalStatus) Help() string {
	return "Collect from SHOW GLOBAL STATUS"
}

// Version of MySQL from which scraper is available.
func (ScrapeGlobalStatus) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (ScrapeGlobalStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	globalStatusRows, err := db.QueryContext(ctx, globalStatusQuery)
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
		"wsrep_evs_repl_latency":   "",
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
				case "data", "dirty", "free", "misc", "old", "total":
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

	if textItems["wsrep_evs_repl_latency"] != "" {
		galeraReplLatencyArray := strings.Split(textItems["wsrep_evs_repl_latency"], "/")

		// check if galeraReplLatencyArray contains all needed values
		if len(galeraReplLatencyArray) == len(replLatencyMap) {
			galeraReplLatencyDesc := prometheus.NewDesc(
				prometheus.BuildFQName(namespace, globalStatus, "wsrep_evs_repl_latency"),
				"PXC/Galera replication latency on group communication.",
				[]string{"aggregator"}, nil,
			)
			for index, label := range replLatencyMap {
				if floatVal, err := strconv.ParseFloat(galeraReplLatencyArray[index], 64); err == nil {
					ch <- prometheus.MustNewConstMetric(
						galeraReplLatencyDesc, prometheus.GaugeValue, floatVal, label,
					)
				}
			}
		}
	}

	return nil
}
