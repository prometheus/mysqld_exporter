// Scrape heartbeat data.

package collector

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// heartbeat is the Metric subsystem we use.
	heartbeat = "heartbeat"
	// heartbeatQuery is the query used to fetch the stored and current
	// timestamps. %s will be replaced by the database and table name.
	// The second column allows gets the server timestamp at the exact same
	// time the query is run.
	heartbeatQuery = "SELECT UNIX_TIMESTAMP(ts), UNIX_TIMESTAMP(NOW(6)), server_id from `%s`.`%s`"
)

var (
	collectHeartbeatDatabase = flag.String(
		"collect.heartbeat.database", "heartbeat",
		"Database from where to collect heartbeat data",
	)
	collectHeartbeatTable = flag.String(
		"collect.heartbeat.table", "heartbeat",
		"Table from where to collect heartbeat data",
	)
)

// Metric descriptors.
var (
	HeartbeatStoredDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, heartbeat, "stored_timestamp_seconds"),
		"Timestamp stored in the heartbeat table.",
		[]string{"server_id"}, nil,
	)
	HeartbeatNowDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, heartbeat, "now_timestamp_seconds"),
		"Timestamp of the current server.",
		[]string{"server_id"}, nil,
	)
)

// ScrapeHeartbeat scrapes from the heartbeat table.
// This is mainly targeting pt-heartbeat, but will work with any heartbeat
// implementation that writes to a table with two columns:
// CREATE TABLE heartbeat (
//  ts                    varchar(26) NOT NULL,
//  server_id             int unsigned NOT NULL PRIMARY KEY,
// );
type ScrapeHeartbeat struct{}

// Name of the Scraper.
func (ScrapeHeartbeat) Name() string {
	return "heartbeat"
}

// Help returns additional information about Scraper.
func (ScrapeHeartbeat) Help() string {
	return "Collect from heartbeat"
}

// Version of MySQL from which scraper is available.
func (ScrapeHeartbeat) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (ScrapeHeartbeat) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	query := fmt.Sprintf(heartbeatQuery, *collectHeartbeatDatabase, *collectHeartbeatTable)
	heartbeatRows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer heartbeatRows.Close()

	var (
		now, ts  sql.RawBytes
		serverId int
	)

	for heartbeatRows.Next() {
		if err := heartbeatRows.Scan(&ts, &now, &serverId); err != nil {
			return err
		}

		tsFloatVal, err := strconv.ParseFloat(string(ts), 64)
		if err != nil {
			return err
		}

		nowFloatVal, err := strconv.ParseFloat(string(now), 64)
		if err != nil {
			return err
		}

		serverId := strconv.Itoa(serverId)

		ch <- prometheus.MustNewConstMetric(
			HeartbeatNowDesc,
			prometheus.GaugeValue,
			nowFloatVal,
			serverId,
		)
		ch <- prometheus.MustNewConstMetric(
			HeartbeatStoredDesc,
			prometheus.GaugeValue,
			tsFloatVal,
			serverId,
		)
	}

	return nil
}
