// Scrape `SHOW SLAVE STATUS`

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

	if slaveStatusRows.Next() {
		// There is either no row in SHOW SLAVE STATUS (if this is not a
		// slave server), or exactly one. In case of multi-source
		// replication, things work very much differently. This code
		// cannot deal with that case.
		slaveCols, err := slaveStatusRows.Columns()
		if err != nil {
			return err
		}

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
		for i, col := range slaveCols {
			if value, ok := parseStatus(*scanArgs[i].(*sql.RawBytes)); ok { // Silently skip unparsable values.
				ch <- prometheus.MustNewConstMetric(
					newDesc(slaveStatus, strings.ToLower(col), "Generic metric from SHOW SLAVE STATUS."),
					prometheus.UntypedValue,
					value,
				)
			}
		}
	}
	return nil
}
