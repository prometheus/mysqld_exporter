// Scrape heartbeat data.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/satori/go.uuid"
)

const (
	// slavehosts is the Metric subsystem we use.
	slavehosts = "slave_hosts"
	// heartbeatQuery is the query used to fetch the stored and current
	// timestamps. %s will be replaced by the database and table name.
	// The second column allows gets the server timestamp at the exact same
	// time the query is run.
	slaveHostsQuery = "SHOW SLAVE HOSTS"
)

// Metric descriptors.
var (
	SlaveHostsInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, heartbeat, "mysql_slave_hosts_info"),
		"Information about running slaves",
		[]string{"server_id", "slave_host", "port", "master_id", "slave_uuid"}, nil,
	)
)

// ScrapeSlaveHosts scrapes metrics about the replicating slaves.
type ScrapeSlaveHosts struct{}

// Name of the Scraper. Should be unique.
func (ScrapeSlaveHosts) Name() string {
	return slavehosts
}

// Help describes the role of the Scraper.
func (ScrapeSlaveHosts) Help() string {
	return "Scrape information from 'SHOW SLAVE HOSTS'"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeSlaveHosts) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	slaveHostsRows, err := db.Query(slaveHostsQuery)
	if err != nil {
		return err
	}
	defer slaveHostsRows.Close()

	// fields of row
	var serverId string
	var host string
	var port string
	var rrrOrMasterId string
	var slaveUuidOrMasterId string

	// Depends on the version of MySQL being scraped
	var masterId string
	var slaveUuid string

	for slaveHostsRows.Next() {
		// Newer versions of mysql have the following
		// 		Server_id, Host, Port, Master_id, Slave_UUID
		// Older versions of mysql have the following
		// 		Server_id, Host, Port, Rpl_recovery_rank, Master_id
		err := slaveHostsRows.Scan(&serverId, &host, &port, &rrrOrMasterId, &slaveUuidOrMasterId)
		if err != nil {
			return err
		}

		// Check to see if slaveUuidOrMasterId resembles a UUID or not
		// to find out if we are using an old version of MySQL
		if _, err = uuid.FromString(slaveUuidOrMasterId); err != nil {
			// We are running an older version of MySQL with no slave UUID
			slaveUuid = ""
			masterId = slaveUuidOrMasterId
		} else {
			// We are running a more recent version of MySQL
			slaveUuid = slaveUuidOrMasterId
			masterId = rrrOrMasterId
		}

		ch <- prometheus.MustNewConstMetric(
			SlaveHostsInfo,
			prometheus.GaugeValue,
			1,
			serverId,
			host,
			port,
			masterId,
			slaveUuid,
		)
	}

	return nil
}
