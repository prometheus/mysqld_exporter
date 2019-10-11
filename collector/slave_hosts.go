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

// Scrape heartbeat data.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
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

// Version of MySQL from which scraper is available.
func (ScrapeSlaveHosts) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeSlaveHosts) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	slaveHostsRows, err := db.QueryContext(ctx, slaveHostsQuery)
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

// check interface
var _ Scraper = ScrapeSlaveHosts{}
