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

package collector

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationGroupMemberStatsQuery = `
	SELECT * FROM performance_schema.replication_group_member_stats WHERE MEMBER_ID=@@server_uuid
`

var (
	// The list of columns we are interesting in.
	// In MySQL 5.7 these are the 4 first columns available. In MySQL 8.x all 8.
	perfReplicationGroupMemberStats = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"COUNT_TRANSACTIONS_IN_QUEUE": {prometheus.GaugeValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_in_queue"),
				"The number of transactions in the queue pending conflict detection checks.", nil, nil)},
		"COUNT_TRANSACTIONS_CHECKED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_checked_total"),
				"The number of transactions that have been checked for conflicts.", nil, nil)},
		"COUNT_CONFLICTS_DETECTED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "conflicts_detected_total"),
				"The number of transactions that have not passed the conflict detection check.", nil, nil)},
		"COUNT_TRANSACTIONS_ROWS_VALIDATING": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_rows_validating_total"),
				"Number of transaction rows which can be used for certification, but have not been garbage collected.", nil, nil)},
		"COUNT_TRANSACTIONS_REMOTE_IN_APPLIER_QUEUE": {prometheus.GaugeValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_remote_in_applier_queue"),
				"The number of transactions that this member has received from the replication group which are waiting to be applied.", nil, nil)},
		"COUNT_TRANSACTIONS_REMOTE_APPLIED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_remote_applied_total"),
				"Number of transactions this member has received from the group and applied.", nil, nil)},
		"COUNT_TRANSACTIONS_LOCAL_PROPOSED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_local_proposed_total"),
				"Number of transactions which originated on this member and were sent to the group.", nil, nil)},
		"COUNT_TRANSACTIONS_LOCAL_ROLLBACK": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, performanceSchema, "transactions_local_rollback_total"),
				"Number of transactions which originated on this member and were rolled back by the group.", nil, nil)},
	}
)

// ScrapePerfReplicationGroupMemberStats collects from `performance_schema.replication_group_member_stats`.
type ScrapePerfReplicationGroupMemberStats struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfReplicationGroupMemberStats) Name() string {
	return performanceSchema + ".replication_group_member_stats"
}

// Help describes the role of the Scraper.
func (ScrapePerfReplicationGroupMemberStats) Help() string {
	return "Collect metrics from performance_schema.replication_group_member_stats"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfReplicationGroupMemberStats) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationGroupMemberStats) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	rows, err := db.QueryContext(ctx, perfReplicationGroupMemberStatsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var columnNames []string
	if columnNames, err = rows.Columns(); err != nil {
		return err
	}

	var scanArgs = make([]interface{}, len(columnNames))
	for i := range scanArgs {
		scanArgs[i] = &sql.RawBytes{}
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return err
		}

		for i, columnName := range columnNames {
			if metric, ok := perfReplicationGroupMemberStats[columnName]; ok {
				value, err := strconv.ParseFloat(string(*scanArgs[i].(*sql.RawBytes)), 64)
				if err != nil {
					return err
				}
				ch <- prometheus.MustNewConstMetric(metric.desc, metric.vtype, value)
			}
		}
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMemberStats{}
