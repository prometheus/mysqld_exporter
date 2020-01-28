// Copyright 2020 The Prometheus Authors
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

// Scrape `information_schema.replica_host_status`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	MySQL "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

const replicaHostQuery = `
	  SELECT SERVER_ID
		   , if(SESSION_ID='MASTER_SESSION_ID','writer','reader') AS ROLE
		   , CPU
		   , MASTER_SLAVE_LATENCY_IN_MICROSECONDS
		   , REPLICA_LAG_IN_MILLISECONDS
		   , LOG_STREAM_SPEED_IN_KiB_PER_SECOND
		   , CURRENT_REPLAY_LATENCY_IN_MICROSECONDS
		FROM information_schema.replica_host_status
	`

// Metric descriptors.
var (
	infoSchemaReplicaHostCpuDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "replica_host_cpu_percent"),
		"The CPU usage as a percentage.",
		[]string{"server_id", "role"}, nil,
	)
	infoSchemaReplicaHostSlaveLatencyDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "replica_host_slave_latency_seconds"),
		"The master-slave latency in seconds.",
		[]string{"server_id", "role"}, nil,
	)
	infoSchemaReplicaHostLagDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "replica_host_lag_seconds"),
		"The replica lag in seconds.",
		[]string{"server_id", "role"}, nil,
	)
	infoSchemaReplicaHostLogStreamSpeedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "replica_host_log_stream_speed"),
		"The log stream speed in kilobytes per second.",
		[]string{"server_id", "role"}, nil,
	)
	infoSchemaReplicaHostReplayLatencyDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "replica_host_replay_latency_seconds"),
		"The current replay latency in seconds.",
		[]string{"server_id", "role"}, nil,
	)
)

// ScrapeReplicaHost collects from `information_schema.replica_host_status`.
type ScrapeReplicaHost struct{}

// Name of the Scraper. Should be unique.
func (ScrapeReplicaHost) Name() string {
	return "info_schema.replica_host"
}

// Help describes the role of the Scraper.
func (ScrapeReplicaHost) Help() string {
	return "Collect metrics from information_schema.replica_host_status"
}

// Version of MySQL from which scraper is available.
func (ScrapeReplicaHost) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeReplicaHost) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	replicaHostRows, err := db.QueryContext(ctx, replicaHostQuery)
	if err != nil {
		if mysqlErr, ok := err.(*MySQL.MySQLError); ok { // Now the error number is accessible directly
			// Check for error 1109: Unknown table
			if mysqlErr.Number == 1109 {
				level.Debug(logger).Log("msg", "information_schema.replica_host_status is not available.")
				return nil
			}
		}
		return err
	}
	defer replicaHostRows.Close()

	var (
		serverId       string
		role           string
		cpu            float64
		slaveLatency   uint64
		replicaLag     float64
		logStreamSpeed float64
		replayLatency  uint64
	)
	for replicaHostRows.Next() {
		if err := replicaHostRows.Scan(
			&serverId,
			&role,
			&cpu,
			&slaveLatency,
			&replicaLag,
			&logStreamSpeed,
			&replayLatency,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			infoSchemaReplicaHostCpuDesc, prometheus.GaugeValue, cpu,
			serverId, role,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaReplicaHostSlaveLatencyDesc, prometheus.GaugeValue, float64(slaveLatency)*0.000001,
			serverId, role,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaReplicaHostLagDesc, prometheus.GaugeValue, replicaLag*0.001,
			serverId, role,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaReplicaHostLogStreamSpeedDesc, prometheus.GaugeValue, logStreamSpeed,
			serverId, role,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaReplicaHostReplayLatencyDesc, prometheus.GaugeValue, float64(replayLatency)*0.000001,
			serverId, role,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapeReplicaHost{}
