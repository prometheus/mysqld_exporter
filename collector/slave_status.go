// Scrape `SHOW SLAVE STATUS`.

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem.
	slaveStatus = "slave_status"
)

var slaveStatusQueries = [2]string{"SHOW ALL SLAVES STATUS", "SHOW SLAVE STATUS"}
var slaveStatusQuerySuffixes = [3]string{" NONBLOCKING", " NOLOCK", ""}

func columnIndex(slaveCols []string, colName string) int {
	for idx := range slaveCols {
		if slaveCols[idx] == colName {
			return idx
		}
	}
	return -1
}

func columnValue(scanArgs []interface{}, slaveCols []string, colName string) string {
	var columnIndex = columnIndex(slaveCols, colName)
	if columnIndex == -1 {
		return ""
	}
	return string(*scanArgs[columnIndex].(*sql.RawBytes))
}

// ScrapeSlaveStatus collects from `SHOW SLAVE STATUS`.
type ScrapeSlaveStatus struct{}

// Name of the Scraper.
func (ScrapeSlaveStatus) Name() string {
	return slaveStatus
}

// Help returns additional information about Scraper.
func (ScrapeSlaveStatus) Help() string {
	return "Collect from SHOW SLAVE STATUS"
}

// Version of MySQL from which scraper is available.
func (ScrapeSlaveStatus) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (ScrapeSlaveStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		slaveStatusRows *sql.Rows
		err             error
	)
	// Try the both syntax for MySQL/Percona and MariaDB
	for _, query := range slaveStatusQueries {
		slaveStatusRows, err = db.QueryContext(ctx, query)
		if err != nil { // MySQL/Percona
			// Leverage lock-free SHOW SLAVE STATUS by guessing the right suffix
			for _, suffix := range slaveStatusQuerySuffixes {
				slaveStatusRows, err = db.QueryContext(ctx, fmt.Sprint(query, suffix))
				if err == nil {
					break
				}
			}
		} else { // MariaDB
			break
		}
	}
	if err != nil {
		return err
	}
	defer slaveStatusRows.Close()

	slaveCols, err := slaveStatusRows.Columns()
	if err != nil {
		return err
	}

	for slaveStatusRows.Next() {
		// As the number of columns varies with mysqld versions,
		// and sql.Scan requires []interface{}, we need to create a
		// slice of pointers to the elements of slaveData.
		scanArgs := make([]interface{}, len(slaveCols))
		for i := range scanArgs {
			scanArgs[i] = &sql.RawBytes{}
		}

		if err := slaveStatusRows.Scan(scanArgs...); err != nil {
			return err
		}

		masterUUID := columnValue(scanArgs, slaveCols, "Master_UUID")
		masterHost := columnValue(scanArgs, slaveCols, "Master_Host")
		channelName := columnValue(scanArgs, slaveCols, "Channel_Name")       // MySQL & Percona
		connectionName := columnValue(scanArgs, slaveCols, "Connection_name") // MariaDB

		for i, col := range slaveCols {
			if value, ok := parseStatus(*scanArgs[i].(*sql.RawBytes)); ok { // Silently skip unparsable values.
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(
						prometheus.BuildFQName(namespace, slaveStatus, strings.ToLower(col)),
						"Generic metric from SHOW SLAVE STATUS.",
						[]string{"master_host", "master_uuid", "channel_name", "connection_name"},
						nil,
					),
					prometheus.UntypedValue,
					value,
					masterHost, masterUUID, channelName, connectionName,
				)
			}
		}
	}
	return nil
}
