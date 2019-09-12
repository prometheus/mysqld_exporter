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

// Scrape `information_schema.client_statistics`.

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

const clientStatQuery = `SELECT * FROM information_schema.client_statistics`

var (
	// Map known client-statistics values to types. Unknown types will be mapped as
	// untyped.
	informationSchemaClientStatisticsTypes = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"TOTAL_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_total_connections"),
				"The number of connections created for this client.",
				[]string{"client"}, nil)},
		"CONCURRENT_CONNECTIONS": {prometheus.GaugeValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_concurrent_connections"),
				"The number of concurrent connections for this client.",
				[]string{"client"}, nil)},
		"CONNECTED_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_connected_time_seconds_total"),
				"The cumulative number of seconds elapsed while there were connections from this client.",
				[]string{"client"}, nil)},
		"BUSY_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_busy_seconds_total"),
				"The cumulative number of seconds there was activity on connections from this client.",
				[]string{"client"}, nil)},
		"CPU_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_cpu_time_seconds_total"),
				"The cumulative CPU time elapsed, in seconds, while servicing this client's connections.",
				[]string{"client"}, nil)},
		"BYTES_RECEIVED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_bytes_received_total"),
				"The number of bytes received from this client’s connections.",
				[]string{"client"}, nil)},
		"BYTES_SENT": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_bytes_sent_total"),
				"The number of bytes sent to this client’s connections.",
				[]string{"client"}, nil)},
		"BINLOG_BYTES_WRITTEN": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_binlog_bytes_written_total"),
				"The number of bytes written to the binary log from this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_READ": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_read_total"),
				"The number of rows read by this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_SENT": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_sent_total"),
				"The number of rows sent by this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_DELETED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_deleted_total"),
				"The number of rows deleted by this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_INSERTED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_inserted_total"),
				"The number of rows inserted by this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_FETCHED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_fetched_total"),
				"The number of rows fetched by this client’s connections.",
				[]string{"client"}, nil)},
		"ROWS_UPDATED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rows_updated_total"),
				"The number of rows updated by this client’s connections.",
				[]string{"client"}, nil)},
		"TABLE_ROWS_READ": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_table_rows_read_total"),
				"The number of rows read from tables by this client’s connections. (It may be different from ROWS_FETCHED.)",
				[]string{"client"}, nil)},
		"SELECT_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_select_commands_total"),
				"The number of SELECT commands executed from this client’s connections.",
				[]string{"client"}, nil)},
		"UPDATE_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_update_commands_total"),
				"The number of UPDATE commands executed from this client’s connections.",
				[]string{"client"}, nil)},
		"OTHER_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_other_commands_total"),
				"The number of other commands executed from this client’s connections.",
				[]string{"client"}, nil)},
		"COMMIT_TRANSACTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_commit_transactions_total"),
				"The number of COMMIT commands issued by this client’s connections.",
				[]string{"client"}, nil)},
		"ROLLBACK_TRANSACTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_rollback_transactions_total"),
				"The number of ROLLBACK commands issued by this client’s connections.",
				[]string{"client"}, nil)},
		"DENIED_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_denied_connections_total"),
				"The number of connections denied to this client.",
				[]string{"client"}, nil)},
		"LOST_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_lost_connections_total"),
				"The number of this client’s connections that were terminated uncleanly.",
				[]string{"client"}, nil)},
		"ACCESS_DENIED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_access_denied_total"),
				"The number of times this client’s connections issued commands that were denied.",
				[]string{"client"}, nil)},
		"EMPTY_QUERIES": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_empty_queries_total"),
				"The number of times this client’s connections sent empty queries to the server.",
				[]string{"client"}, nil)},
		"TOTAL_SSL_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_total_ssl_connections_total"),
				"The number of times this client’s connections connected using SSL to the server.",
				[]string{"client"}, nil)},
		"MAX_STATEMENT_TIME_EXCEEDED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "client_statistics_max_statement_time_exceeded_total"),
				"The number of times a statement was aborted, because it was executed longer than its MAX_STATEMENT_TIME threshold.",
				[]string{"client"}, nil)},
	}
)

// ScrapeClientStat collects from `information_schema.client_statistics`.
type ScrapeClientStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeClientStat) Name() string {
	return "info_schema.clientstats"
}

// Help describes the role of the Scraper.
func (ScrapeClientStat) Help() string {
	return "If running with userstat=1, set to true to collect client statistics"
}

// Version of MySQL from which scraper is available.
func (ScrapeClientStat) Version() float64 {
	return 5.5
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeClientStat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var varName, varVal string
	err := db.QueryRowContext(ctx, userstatCheckQuery).Scan(&varName, &varVal)
	if err != nil {
		level.Debug(logger).Log("msg", "Detailed client stats are not available.")
		return nil
	}
	if varVal == "OFF" {
		level.Debug(logger).Log("msg", "MySQL variable is OFF.", "var", varName)
		return nil
	}

	informationSchemaClientStatisticsRows, err := db.QueryContext(ctx, clientStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaClientStatisticsRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaClientStatisticsRows.Columns()
	if err != nil {
		return err
	}

	var (
		client             string                                // Holds the client name, which should be in column 0.
		clientStatData     = make([]float64, len(columnNames)-1) // 1 less because of the client column.
		clientStatScanArgs = make([]interface{}, len(columnNames))
	)

	clientStatScanArgs[0] = &client
	for i := range clientStatData {
		clientStatScanArgs[i+1] = &clientStatData[i]
	}

	for informationSchemaClientStatisticsRows.Next() {
		if err := informationSchemaClientStatisticsRows.Scan(clientStatScanArgs...); err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// cient, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metricType, ok := informationSchemaClientStatisticsTypes[columnName]; ok {
				ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientStatData[idx]), client)
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, fmt.Sprintf("client_statistics_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"client"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(clientStatData[idx]), client)
			}
		}
	}
	return nil
}

// check interface
var _ Scraper = ScrapeClientStat{}
