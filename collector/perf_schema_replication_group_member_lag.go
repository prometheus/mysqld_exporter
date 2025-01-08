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

package collector

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationGroupMemberLagQuery = `
	SELECT IF(
						applier_coordinator_status.SERVICE_STATE = 'OFF'
					OR conn_status.SERVICE_STATE = 'OFF',
						99999999,
						IF(
										GTID_SUBTRACT(conn_status.LAST_QUEUED_TRANSACTION,
														applier_status.LAST_APPLIED_TRANSACTION) = ''
									OR UNIX_TIMESTAMP(applier_status.APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP) =
										0,
										0,
										TIME_TO_SEC(TIMEDIFF(
												NOW(6),
												applier_status.APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP
											))
							)
			) AS replication_group_member_lag
	FROM performance_schema.replication_connection_status AS conn_status
			JOIN performance_schema.replication_applier_status_by_worker AS applier_status
				ON applier_status.channel_name = conn_status.channel_name
			JOIN performance_schema.replication_applier_status_by_coordinator AS applier_coordinator_status
				ON applier_coordinator_status.channel_name = conn_status.channel_name
	WHERE conn_status.channel_name = 'group_replication_applier'
	ORDER BY IF(GTID_SUBTRACT(conn_status.LAST_QUEUED_TRANSACTION,
							applier_status.LAST_APPLIED_TRANSACTION) = ''
					OR UNIX_TIMESTAMP(applier_status.APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP) = 0,
				'1-IDLE', '0-EXECUTING') ASC,
			applier_status.APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP ASC
	LIMIT 1;
	`

// ScrapeReplicationGroupMembers collects from `performance_schema.replication_group_members`.
type ScrapePerfReplicationGroupMemberLag struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfReplicationGroupMemberLag) Name() string {
	return performanceSchema + ".replication_group_member_lag"
}

// Help describes the role of the Scraper.
func (ScrapePerfReplicationGroupMemberLag) Help() string {
	return "Collect the replication lag according to applier queue from performance_schema group replication tables"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfReplicationGroupMemberLag) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationGroupMemberLag) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()
	var lag uint64
	err := db.QueryRowContext(ctx, perfReplicationGroupMemberLagQuery).Scan(&lag)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "replication_group_member_lag"),
			"Group replication lag in seconds", nil, nil),
		prometheus.GaugeValue, float64(lag),
	)
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMemberLag{}
