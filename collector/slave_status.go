// Scrape `SHOW SLAVE STATUS`.

package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	// Subsystem.
	slaveStatus = "slave_status"
)

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

var (
	maria55              = regexp.MustCompile(`^5\.[1-5]`)         // support only SHOW SLAVE STATUS
	perconaNolock55      = regexp.MustCompile(`^5\.5`)             // support SHOW SLAVE STATUS NOLOCK
	perconaNolock56      = regexp.MustCompile(`^5\.6\.1[1-9]`)     // support SHOW SLAVE STATUS NOLOCK
	perconaNonblocking56 = regexp.MustCompile(`^5\.6\.[2-9][0-9]`) // support SHOW SLAVE STATUS NONBLOCKING
)

// chooseQuery chooses a query to get slave status by database's distro and version.
func chooseQuery(ctx context.Context, db *sql.DB) (string, error) {
	var (
		version        string
		versionComment string
	)

	if err := db.QueryRowContext(ctx, "SELECT @@version, @@version_comment").Scan(&version, &versionComment); err != nil {
		return "", err
	}
	log.Infof("database version %s, distro %s", version, versionComment)

	query := "SHOW SLAVE STATUS"
	switch {
	case strings.Contains(strings.ToLower(versionComment), "maria") && !maria55.MatchString(version):
		query = "SHOW ALL SLAVES STATUS"
	case strings.Contains(strings.ToLower(versionComment), "percona") && (perconaNolock56.MatchString(version) || perconaNolock55.MatchString(version)):
		// https://www.percona.com/doc/percona-server/5.6/reliability/show_slave_status_nolock.html
		// > 5.6.11-60.3: Feature ported from Percona Server for MySQL 5.5.
		query = "SHOW SLAVE STATUS NOLOCK" // Percona Server v >= 5.6.11
	case strings.Contains(strings.ToLower(versionComment), "percona") && perconaNonblocking56.MatchString(version):
		// https://www.percona.com/doc/percona-server/5.6/reliability/show_slave_status_nolock.html
		// > 5.6.20-68.0: Percona Server for MySQL implemented the NONBLOCKING syntax from MySQL 5.7 and deprecated the NOLOCK syntax.
		// > 5.6.27-76.0: SHOW SLAVE STATUS NOLOCK syntax in 5.6 has been undeprecated. Both SHOW SLAVE STATUS NOLOCK and SHOW SLAVE STATUS NONBLOCKING are now supported.
		query = "SHOW SLAVE STATUS NONBLOCKING"
	}
	return query, nil
}

// Scrape collects data.
func (ScrapeSlaveStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	var (
		slaveStatusRows *sql.Rows
		err             error
	)

	query, err := chooseQuery(ctx, db)
	if err != nil {
		return err
	}

	if slaveStatusRows, err = db.QueryContext(ctx, query); err != nil {
		log.Errorf("cannot scrape status with a chosen query %q: %v", query, err)
		// fallback to the common query.
		query = "SHOW SLAVE STATUS"
		if slaveStatusRows, err = db.QueryContext(ctx, query); err != nil {
			log.Errorf("cannot scrape status by the common query: %v", err)
			return err
		}
	}

	log.Infof("Successfully scraped status with query: %s", query)

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
