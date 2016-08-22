// Scrape `SHOW GLOBAL VARIABLES`.

package collector

import (
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Metric subsystem
	globalVariables = "global_variables"
	// Metric SQL Queries.
	globalVariablesQuery = `SHOW GLOBAL VARIABLES`
)

// ScrapeGlobalVariables collects from `SHOW GLOBAL VARIABLES`.
func ScrapeGlobalVariables(db *sql.DB, ch chan<- prometheus.Metric) error {
	globalVariablesRows, err := db.Query(globalVariablesQuery)
	if err != nil {
		return err
	}
	defer globalVariablesRows.Close()

	var key string
	var val sql.RawBytes
	var textItems = map[string]string{
		"innodb_version":     "",
		"version":            "",
		"version_comment":    "",
		"wsrep_cluster_name": "",
	}

	for globalVariablesRows.Next() {
		if err := globalVariablesRows.Scan(&key, &val); err != nil {
			return err
		}
		key = strings.ToLower(key)
		if floatVal, ok := parseStatus(val); ok {
			ch <- prometheus.MustNewConstMetric(
				newDesc(globalVariables, key, "Generic gauge metric from SHOW GLOBAL VARIABLES."),
				prometheus.GaugeValue,
				floatVal,
			)
			continue
		} else if _, ok := textItems[key]; ok {
			textItems[key] = string(val)
		}
	}

	// mysql_version_info metric.
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "version", "info"), "MySQL version and distribution.",
			[]string{"innodb_version", "version", "version_comment"}, nil),
		prometheus.GaugeValue, 1, textItems["innodb_version"], textItems["version"], textItems["version_comment"],
	)

	// mysql_galera_variables_info metric.
	if textItems["wsrep_cluster_name"] != "" {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "galera", "variables_info"), "PXC/Galera variables information.",
				[]string{"wsrep_cluster_name"}, nil),
			prometheus.GaugeValue, 1, textItems["wsrep_cluster_name"],
		)
	}

	return nil
}
