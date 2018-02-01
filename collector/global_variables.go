// Scrape `SHOW GLOBAL VARIABLES`.

package collector

import (
	"database/sql"
	"regexp"
	"strconv"
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
type ScrapeGlobalVariables struct{}

// Name of the Scraper. Should be unique.
func (ScrapeGlobalVariables) Name() string {
	return globalVariables
}

// Help describes the role of the Scraper.
func (ScrapeGlobalVariables) Help() string {
	return "Collect from SHOW GLOBAL VARIABLES"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeGlobalVariables) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	globalVariablesRows, err := db.Query(globalVariablesQuery)
	if err != nil {
		return err
	}
	defer globalVariablesRows.Close()

	var key string
	var val sql.RawBytes
	var textItems = map[string]string{
		"innodb_version":         "",
		"version":                "",
		"version_comment":        "",
		"wsrep_cluster_name":     "",
		"wsrep_provider_options": "",
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

	// mysql_galera_gcache_size_bytes metric.
	if textItems["wsrep_provider_options"] != "" {
		ch <- prometheus.MustNewConstMetric(
			newDesc("galera", "gcache_size_bytes", "PXC/Galera gcache size."),
			prometheus.GaugeValue,
			parseWsrepProviderOptions(textItems["wsrep_provider_options"]),
		)
	}

	return nil
}

// parseWsrepProviderOptions parse wsrep_provider_options to get gcache.size in bytes.
func parseWsrepProviderOptions(opts string) float64 {
	var val float64
	r, _ := regexp.Compile(`gcache.size = (\d+)([MG]?);`)
	data := r.FindStringSubmatch(opts)
	if data == nil {
		return 0
	}

	val, _ = strconv.ParseFloat(data[1], 64)
	switch data[2] {
	case "M":
		val = val * 1024 * 1024
	case "G":
		val = val * 1024 * 1024 * 1024
	}

	return val
}
