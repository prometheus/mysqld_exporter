// Scrape `SHOW SLAVE STATUS`.

package collector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem.
	slaveStatus = "slave_status"
	// Query.
	slaveStatusQuery = `SHOW SLAVE STATUS`
)

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
func ScrapeSlaveStatus(db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		slaveStatusRows *sql.Rows
		err             error
	)
	// Leverage lock-free SHOW SLAVE STATUS by guessing the right suffix
	for _, suffix := range slaveStatusQuerySuffixes {
		slaveStatusRows, err = db.Query(fmt.Sprint(slaveStatusQuery, suffix))
		if err == nil {
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
		channelName := columnValue(scanArgs, slaveCols, "Channel_Name")

		for i, col := range slaveCols {
			if value, ok := parseStatus(*scanArgs[i].(*sql.RawBytes)); ok { // Silently skip unparsable values.
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(
						prometheus.BuildFQName(namespace, slaveStatus, strings.ToLower(col)),
						"Generic metric from SHOW SLAVE STATUS.",
						[]string{"master_uuid", "master_host", "channel_name"},
						nil,
					),
					prometheus.UntypedValue,
					value,
					masterUUID, masterHost, channelName,
				)
			}
		}
	}
	return nil
}
