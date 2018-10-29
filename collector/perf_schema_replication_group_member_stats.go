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

	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationGroupMemeberStatsQuery = `
	SELECT MEMBER_ID,COUNT_TRANSACTIONS_IN_QUEUE,COUNT_TRANSACTIONS_CHECKED,COUNT_CONFLICTS_DETECTED,COUNT_TRANSACTIONS_ROWS_VALIDATING
	  FROM performance_schema.replication_group_member_stats
	`

// Metric descriptors.
var (
	performanceSchemaReplicationGroupMemberStatsTransInQueueDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "transaction_in_queue"),
		"The number of transactions in the queue pending conflict detection checks. Once the "+
			"transactions have been checked for conflicts, if they pass the check, they are queued to be applied as well.",
		[]string{"member_id"}, nil,
	)
	performanceSchemaReplicationGroupMemberStatsTransCheckedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "transaction_checked"),
		"The number of transactions that have been checked for conflicts.",
		[]string{"member_id"}, nil,
	)
	performanceSchemaReplicationGroupMemberStatsConflictsDetectedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "conflicts_detected"),
		"The number of transactions that did not pass the conflict detection check.",
		[]string{"member_id"}, nil,
	)
	performanceSchemaReplicationGroupMemberStatsTransRowValidatingDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "transaction_rows_validating"),
		"The current size of the conflict detection database (against which each transaction is certified).",
		[]string{"member_id"}, nil,
	)
)

// ScrapeReplicationGroupMemberStats collects from `performance_schema.replication_group_member_stats`.
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
func (ScrapePerfReplicationGroupMemberStats) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	perfReplicationGroupMemeberStatsRows, err := db.QueryContext(ctx, perfReplicationGroupMemeberStatsQuery)
	if err != nil {
		return err
	}
	defer perfReplicationGroupMemeberStatsRows.Close()

	var (
		memberId                                                string
		countTransactionsInQueue, countTransactionsChecked      uint64
		countConflictsDetected, countTransactionsRowsValidating uint64
	)

	for perfReplicationGroupMemeberStatsRows.Next() {
		if err := perfReplicationGroupMemeberStatsRows.Scan(
			&memberId, &countTransactionsInQueue, &countTransactionsChecked,
			&countConflictsDetected, &countTransactionsRowsValidating,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationGroupMemberStatsTransInQueueDesc, prometheus.CounterValue, float64(countTransactionsInQueue),
			memberId,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationGroupMemberStatsTransCheckedDesc, prometheus.CounterValue, float64(countTransactionsChecked),
			memberId,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationGroupMemberStatsConflictsDetectedDesc, prometheus.CounterValue, float64(countConflictsDetected),
			memberId,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationGroupMemberStatsTransRowValidatingDesc, prometheus.CounterValue, float64(countTransactionsRowsValidating),
			memberId,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationGroupMemberStats{}
