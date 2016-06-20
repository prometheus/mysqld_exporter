// Scrape `information_schema.client_statistics`.

package collector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const clientStatQuery = `
			SELECT
				CLIENT,
				TOTAL_CONNECTIONS,
				CONCURRENT_CONNECTIONS,
				CONNECTED_TIME,
				BUSY_TIME,
				CPU_TIME,
				BYTES_RECEIVED,
				BYTES_SENT,
				BINLOG_BYTES_WRITTEN,
				ROWS_READ,
				ROWS_SENT,
				ROWS_DELETED,
				ROWS_INSERTED,
				ROWS_UPDATED,
				SELECT_COMMANDS,
				UPDATE_COMMANDS,
				OTHER_COMMANDS,
				COMMIT_TRANSACTIONS,
				ROLLBACK_TRANSACTIONS,
				DENIED_CONNECTIONS,
				LOST_CONNECTIONS,
				ACCESS_DENIED,
				EMPTY_QUERIES
			FROM information_schema.client_statistics`

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
	}
)

// ScrapeClientStat collects from `information_schema.client_statistics`.
func ScrapeClientStat(db *sql.DB, ch chan<- prometheus.Metric) error {
	informationSchemaClientStatisticsRows, err := db.Query(clientStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaClientStatisticsRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	var columnNames []string
	columnNames, err = informationSchemaClientStatisticsRows.Columns()
	if err != nil {
		return err
	}

	var client string                                        // Holds the client name, which should be in column 0.
	var clientStatData = make([]float64, len(columnNames)-1) // 1 less because of the client column.
	var clientStatScanArgs = make([]interface{}, len(columnNames))
	clientStatScanArgs[0] = &client
	for i := range clientStatData {
		clientStatScanArgs[i+1] = &clientStatData[i]
	}

	for informationSchemaClientStatisticsRows.Next() {
		err = informationSchemaClientStatisticsRows.Scan(clientStatScanArgs...)
		if err != nil {
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
