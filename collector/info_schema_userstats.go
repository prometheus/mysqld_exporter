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

// Scrape `information_schema.user_statistics`.

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

const userStatQuery = `SELECT * FROM information_schema.user_statistics`

var (
	informationSchemaUserStatistics = map[string][]string{
		"TOTAL_CONNECTIONS":           []string{"user_statistics_total_connections", "The number of connections created for this user."},
		"CONCURRENT_CONNECTIONS":      []string{"user_statistics_concurrent_connections", "The number of concurrent connections for this user."},
		"CONNECTED_TIME":              []string{"user_statistics_connected_time_seconds_total", "The cumulative number of seconds elapsed while there were connections from this user."},
		"BUSY_TIME":                   []string{"user_statistics_busy_seconds_total", "The cumulative number of seconds there was activity on connections from this user."},
		"CPU_TIME":                    []string{"user_statistics_cpu_time_seconds_total", "The cumulative CPU time elapsed, in seconds, while servicing this user's connections."},
		"BYTES_RECEIVED":              []string{"user_statistics_bytes_received_total", "The number of bytes received from this user’s connections."},
		"BYTES_SENT":                  []string{"user_statistics_bytes_sent_total", "The number of bytes sent to this user’s connections."},
		"BINLOG_BYTES_WRITTEN":        []string{"user_statistics_binlog_bytes_written_total", "The number of bytes written to the binary log from this user’s connections."},
		"ROWS_READ":                   []string{"user_statistics_rows_read_total", "The number of rows read by this user’s connections."},
		"ROWS_SENT":                   []string{"user_statistics_rows_sent_total", "The number of rows sent by this user’s connections."},
		"ROWS_DELETED":                []string{"user_statistics_rows_deleted_total", "The number of rows deleted by this user’s connections."},
		"ROWS_INSERTED":               []string{"user_statistics_rows_inserted_total", "The number of rows inserted by this user’s connections."},
		"ROWS_FETCHED":                []string{"user_statistics_rows_fetched_total", "The number of rows fetched by this user’s connections."},
		"ROWS_UPDATED":                []string{"user_statistics_rows_updated_total", "The number of rows updated by this user’s connections."},
		"TABLE_ROWS_READ":             []string{"user_statistics_table_rows_read_total", "The number of rows read from tables by this user’s connections. (It may be different from ROWS_FETCHED)."},
		"SELECT_COMMANDS":             []string{"user_statistics_select_commands_total", "The number of SELECT commands executed from this user’s connections."},
		"UPDATE_COMMANDS":             []string{"user_statistics_update_commands_total", "The number of UPDATE commands executed from this user’s connections."},
		"OTHER_COMMANDS":              []string{"user_statistics_other_commands_total", "The number of other commands executed from this user’s connections."},
		"COMMIT_TRANSACTIONS":         []string{"user_statistics_commit_transactions_total", "The number of COMMIT commands issued by this user’s connections."},
		"ROLLBACK_TRANSACTIONS":       []string{"user_statistics_rollback_transactions_total", "The number of ROLLBACK commands issued by this user’s connections."},
		"DENIED_CONNECTIONS":          []string{"user_statistics_denied_connections_total", "The number of connections denied to this user."},
		"LOST_CONNECTIONS":            []string{"user_statistics_lost_connections_total", "The number of this user’s connections that were terminated uncleanly."},
		"ACCESS_DENIED":               []string{"user_statistics_access_denied_total", "The number of times this user’s connections issued commands that were denied."},
		"EMPTY_QUERIES":               []string{"user_statistics_empty_queries_total", "The number of times this user’s connections sent empty queries to the server."},
		"TOTAL_SSL_CONNECTIONS":       []string{"user_statistics_total_ssl_connections_total", "The number of times this user’s connections connected using SSL to the server."},
		"MAX_STATEMENT_TIME_EXCEEDED": []string{"user_statistics_max_statement_time_exceeded_total", "The number of times a statement was aborted, because it was executed longer than its MAX_STATEMENT_TIME threshold."},
	}
)

// ScrapeUserStat collects from `information_schema.user_statistics`.
type ScrapeUserStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeUserStat) Name() string {
	return "info_schema.userstats"
}

// Help describes the role of the Scraper.
func (ScrapeUserStat) Help() string {
	return "If running with userstat=1, set to true to collect user statistics"
}

// Version of MySQL from which scraper is available.
func (ScrapeUserStat) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeUserStat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, constLabels prometheus.Labels, logger log.Logger) error {
	var varName, varVal string
	err := db.QueryRowContext(ctx, userstatCheckQuery).Scan(&varName, &varVal)
	if err != nil {
		level.Debug(logger).Log("msg", "Detailed user stats are not available.")
		return nil
	}
	if varVal == "OFF" {
		level.Debug(logger).Log("msg", "MySQL variable is OFF.", "var", varName)
		return nil
	}

	informationSchemaUserStatisticsRows, err := db.QueryContext(ctx, userStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaUserStatisticsRows.Close()

	// The user column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the user column, userStatData[0] maps to columnNames[1] when reading off the metrics
	// (because userStatScanArgs is mapped as [ &user, &userData[0], &userData[1] ... &userdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	var columnNames []string
	columnNames, err = informationSchemaUserStatisticsRows.Columns()
	if err != nil {
		return err
	}

	var user string                                        // Holds the username, which should be in column 0.
	var userStatData = make([]float64, len(columnNames)-1) // 1 less because of the user column.
	var userStatScanArgs = make([]interface{}, len(columnNames))
	userStatScanArgs[0] = &user
	for i := range userStatData {
		userStatScanArgs[i+1] = &userStatData[i]
	}

	for informationSchemaUserStatisticsRows.Next() {
		err = informationSchemaUserStatisticsRows.Scan(userStatScanArgs...)
		if err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// user, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metric, ok := informationSchemaUserStatistics[columnName]; ok {
				metricType := prometheus.CounterValue
				if columnName == "CONCURRENT_CONNECTIONS" {
					metricType = prometheus.GaugeValue
				}
				desc := newDescLabels(informationSchema, metric[0], metric[1], constLabels, []string{"user"})
				ch <- prometheus.MustNewConstMetric(desc, metricType, float64(userStatData[idx]), user)
			} else {
				// Unknown metric. Report as untyped.
				desc := newDescLabels(informationSchema, fmt.Sprintf("user_statistics_%s", strings.ToLower(columnName)), fmt.Sprintf("Unsupported metric from column %s", columnName), constLabels, []string{"user"})
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(userStatData[idx]), user)
			}
		}
	}
	return nil
}

// check interface
var _ Scraper = ScrapeUserStat{}
