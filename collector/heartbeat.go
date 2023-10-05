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
	"fmt"
	"log/slog"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// heartbeat is the Metric subsystem we use.
	heartbeat = "heartbeat"
	// heartbeatQuery is the query used to fetch the stored and current
	// timestamps. %s will be replaced by the database and table name.
	// The second column allows gets the server timestamp at the exact same
	// time the query is run.
	heartbeatQuery = "SELECT UNIX_TIMESTAMP(ts), UNIX_TIMESTAMP(%s), server_id from `%s`.`%s`"
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
//
//	ts                    varchar(26) NOT NULL,
//	server_id             int unsigned NOT NULL PRIMARY KEY,
//
// );
type ScrapeHeartbeat struct {
	Database string
	Table    string
	UTC      bool
}

// Name of the Scraper. Should be unique.
func (ScrapeHeartbeat) Name() string {
	return "heartbeat"
}

// Help describes the role of the Scraper.
func (ScrapeHeartbeat) Help() string {
	return "Collect from heartbeat"
}

// Version of MySQL from which scraper is available.
func (ScrapeHeartbeat) Version() float64 {
	return 5.1
}

// RegisterFlags adds flags to configure the Scraper.
func (s *ScrapeHeartbeat) RegisterFlags(application *kingpin.Application) {
	application.Flag(
		"collect.heartbeat.database",
		"Database from where to collect heartbeat data",
	).Default("heartbeat").StringVar(&s.Database)
	application.Flag(
		"collect.heartbeat.table",
		"Table from where to collect heartbeat data",
	).Default("heartbeat").StringVar(&s.Table)
	application.Flag(
		"collect.heartbeat.utc",
		"Use UTC for timestamps of the current server (`pt-heartbeat` is called with `--utc`)",
	).BoolVar(&s.UTC)
}

// nowExpr returns a current timestamp expression.
func (s ScrapeHeartbeat) nowExpr() string {
	if s.UTC {
		return "UTC_TIMESTAMP(6)"
	}
	return "NOW(6)"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (s ScrapeHeartbeat) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()
	query := fmt.Sprintf(heartbeatQuery, s.nowExpr(), s.Database, s.Table)
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

// check interface
var _ Scraper = ScrapeHeartbeat{}
