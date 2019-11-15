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

// Scrape `SHOW GLOBAL STATUS`.

package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
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

// Version of MySQL from which scraper is available.
func (ScrapeGlobalStatus) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeGlobalStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, constLabels prometheus.Labels, logger log.Logger) error {
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
			key = validPrometheusName(key)
			match := globalStatusRE.FindStringSubmatch(key)
			if match == nil {
				ch <- prometheus.MustNewConstMetric(
					newDesc(globalStatus, key, "Generic metric from SHOW GLOBAL STATUS.", constLabels),
					prometheus.UntypedValue,
					floatVal,
				)
				continue
			}
			switch match[1] {
			case "com":
				ch <- prometheus.MustNewConstMetric(
					newDescLabels(globalStatus, "commands_total", "Total number of executed MySQL commands.", constLabels, []string{"command"}),
					prometheus.CounterValue, floatVal, match[2],
				)
			case "handler":
				ch <- prometheus.MustNewConstMetric(
					newDescLabels(globalStatus, "handlers_total", "Total number of executed MySQL handlers.", constLabels, []string{"handler"}),
					prometheus.CounterValue, floatVal, match[2],
				)
			case "connection_errors":
				ch <- prometheus.MustNewConstMetric(
					newDescLabels(globalStatus, "connection_errors_total", "Total number of MySQL connection errors.", constLabels, []string{"error"}),
					prometheus.CounterValue, floatVal, match[2],
				)
			case "innodb_buffer_pool_pages":
				switch match[2] {
				case "data", "free", "misc", "old":
					ch <- prometheus.MustNewConstMetric(
						newDescLabels(globalStatus, "buffer_pool_pages", "Innodb buffer pool pages by state.", constLabels, []string{"state"}),
						prometheus.GaugeValue, floatVal, match[2],
					)
				case "dirty":
					ch <- prometheus.MustNewConstMetric(
						newDesc(globalStatus, "buffer_pool_dirty_pages", "Innodb buffer pool dirty pages.", constLabels),
						prometheus.GaugeValue, floatVal,
					)
				case "total":
					continue
				default:
					ch <- prometheus.MustNewConstMetric(
						newDescLabels(globalStatus, "buffer_pool_page_changes_total", "Innodb buffer pool page state changes.", constLabels, []string{"operation"}),
						prometheus.CounterValue, floatVal, match[2],
					)
				}
			case "innodb_rows":
				ch <- prometheus.MustNewConstMetric(
					newDescLabels(globalStatus, "innodb_row_ops_total", "Total number of MySQL InnoDB row operations.", constLabels, []string{"operation"}),
					prometheus.CounterValue, floatVal, match[2],
				)
			case "performance_schema":
				ch <- prometheus.MustNewConstMetric(
					newDescLabels(globalStatus, "performance_schema_lost_total", "Total number of MySQL instrumentations that could not be loaded or created due to memory constraints.", constLabels, []string{"instrumentation"}),
					prometheus.CounterValue, floatVal, match[2],
				)
			}
		} else if _, ok := textItems[key]; ok {
			textItems[key] = string(val)
		}
	}

	// mysql_galera_variables_info metric.
	if textItems["wsrep_local_state_uuid"] != "" {
		ch <- prometheus.MustNewConstMetric(
			newDescLabels("galera", "status_info", "PXC/Galera status information.", constLabels, []string{"wsrep_local_state_uuid", "wsrep_cluster_state_uuid", "wsrep_provider_version"}),
			prometheus.GaugeValue, 1, textItems["wsrep_local_state_uuid"], textItems["wsrep_cluster_state_uuid"], textItems["wsrep_provider_version"],
		)
	}

	// mysql_galera_evs_repl_latency
	if textItems["wsrep_evs_repl_latency"] != "" {

		type evsValue struct {
			name  string
			value float64
			index int
			help  string
		}

		evsMap := []evsValue{
			evsValue{name: "min_seconds", value: 0, index: 0, help: "PXC/Galera group communication latency. Min value."},
			evsValue{name: "avg_seconds", value: 0, index: 1, help: "PXC/Galera group communication latency. Avg value."},
			evsValue{name: "max_seconds", value: 0, index: 2, help: "PXC/Galera group communication latency. Max value."},
			evsValue{name: "stdev", value: 0, index: 3, help: "PXC/Galera group communication latency. Standard Deviation."},
			evsValue{name: "sample_size", value: 0, index: 4, help: "PXC/Galera group communication latency. Sample Size."},
		}

		evsParsingSuccess := true
		values := strings.Split(textItems["wsrep_evs_repl_latency"], "/")

		if len(evsMap) == len(values) {
			for i, v := range evsMap {
				evsMap[i].value, err = strconv.ParseFloat(values[v.index], 64)
				if err != nil {
					evsParsingSuccess = false
				}
			}

			if evsParsingSuccess {
				for _, v := range evsMap {
					ch <- prometheus.MustNewConstMetric(
						newDesc("galera_evs_repl_latency", v.name, v.help, constLabels),
						prometheus.GaugeValue, v.value)
				}
			}
		}
	}

	return nil
}

// check interface
var _ Scraper = ScrapeGlobalStatus{}
