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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationApplierStatsByWorkerQuery = `
	SELECT CHANNEL_NAME, WORKER_ID, 
		LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP, LAST_APPLIED_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP, LAST_APPLIED_TRANSACTION_START_APPLY_TIMESTAMP, LAST_APPLIED_TRANSACTION_END_APPLY_TIMESTAMP,
		APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP, APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP, APPLYING_TRANSACTION_START_APPLY_TIMESTAMP,
		(NOW()-APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP) as REPLICAION_LAG
    FROM performance_schema.replication_applier_status_by_worker
	`

// Metric descriptors.
var (
	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionOriginalCommitTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_original_commit_timestamp"),
		"A timestamp shows when the last transaction applied by this worker was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_immediate_commit_timestamp"),
		"A timestamp shows when the last transaction applied by this worker was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplyTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_start_apply_timestamp"),
		"A timestamp shows when this worker started applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplyTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_end_apply_timestamp"),
		"A shows when this worker finished applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_original_commit_timestamp"),
		"A timestamp that shows when the transaction this worker is currently applying was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_immediate_commit_timestamp"),
		"A timestamp shows when the transaction this worker is currently applying was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplyTimestampDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_start_apply_timestamp"),
		"A timestamp shows when this worker started its first attempt to apply the transaction that is currently being applied.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerReplicaionLagDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "replicaion_lag"),
		"Replication lag in second. Calculated by (NOW()-APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP)",
		[]string{"channel_name", "member_id"}, nil,
	)
)

// ScrapePerfReplicationApplierStatsByWorker collects from `performance_schema.replication_applier_status_by_worker`.
type ScrapePerfReplicationApplierStatsByWorker struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfReplicationApplierStatsByWorker) Name() string {
	return performanceSchema + ".replication_applier_status_by_worker"
}

// Help describes the role of the Scraper.
func (ScrapePerfReplicationApplierStatsByWorker) Help() string {
	return "Collect metrics from performance_schema.replication_applier_status_by_worker"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfReplicationApplierStatsByWorker) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationApplierStatsByWorker) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	perfReplicationApplierStatsByWorkerRows, err := db.QueryContext(ctx, perfReplicationApplierStatsByWorkerQuery)
	if err != nil {
		return err
	}
	defer perfReplicationApplierStatsByWorkerRows.Close()

	var (
		channelName, workerId                                                                                         string
		lastAppliedTransactionOriginalCommitTimestamp, lastAppliedTransactionImmediateCommitTimestamp                 time.Time
		lastAppliedTransactionStartApplyTimestamp, lastAppliedTransactionEndApplyTimestamp                            time.Time
		applyingTransactionOriginalCommitTimestamp, applyingTransactionImmediateCommitTimestamp                       time.Time
		applyingTransactionStartApplyTimestamp                                                                        time.Time
		replicationLag                                                                                                []uint8
		lastAppliedTransactionOriginalCommit, lastAppliedTransactionImmediateCommit, lastAppliedTransactionStartApply float64
		lastAppliedTransactionEndApply, applyingTransactionOriginalCommit, applyingTransactionImmediateCommit         float64
		applyingTransactionStartApply, lag                                                                            float64
	)

	for perfReplicationApplierStatsByWorkerRows.Next() {
		if err := perfReplicationApplierStatsByWorkerRows.Scan(
			&channelName, &workerId,
			&lastAppliedTransactionOriginalCommitTimestamp, &lastAppliedTransactionImmediateCommitTimestamp,
			&lastAppliedTransactionStartApplyTimestamp, &lastAppliedTransactionEndApplyTimestamp,
			&applyingTransactionOriginalCommitTimestamp, &applyingTransactionImmediateCommitTimestamp,
			&applyingTransactionStartApplyTimestamp, &replicationLag,
		); err != nil {
			return err
		}

		// Check if the value is 0, use a real 0
		if lastAppliedTransactionOriginalCommitTimestamp.Nanosecond() != 0 {
			lastAppliedTransactionOriginalCommit = float64(lastAppliedTransactionOriginalCommitTimestamp.Unix())
		} else {
			lastAppliedTransactionOriginalCommit = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionOriginalCommitTimestampDesc, prometheus.GaugeValue, lastAppliedTransactionOriginalCommit,
			channelName, workerId,
		)

		if lastAppliedTransactionImmediateCommitTimestamp.Nanosecond() != 0 {
			lastAppliedTransactionImmediateCommit = float64(lastAppliedTransactionImmediateCommitTimestamp.Unix())
		} else {
			lastAppliedTransactionImmediateCommit = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitTimestampDesc, prometheus.GaugeValue, lastAppliedTransactionImmediateCommit,
			channelName, workerId,
		)

		if lastAppliedTransactionStartApplyTimestamp.Nanosecond() != 0 {
			lastAppliedTransactionStartApply = float64(lastAppliedTransactionStartApplyTimestamp.Unix())
		} else {
			lastAppliedTransactionStartApply = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplyTimestampDesc, prometheus.GaugeValue, lastAppliedTransactionStartApply,
			channelName, workerId,
		)

		if lastAppliedTransactionEndApplyTimestamp.Nanosecond() != 0 {
			lastAppliedTransactionEndApply = float64(lastAppliedTransactionEndApplyTimestamp.Unix())
		} else {
			lastAppliedTransactionEndApply = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplyTimestampDesc, prometheus.GaugeValue, lastAppliedTransactionEndApply,
			channelName, workerId,
		)

		if applyingTransactionOriginalCommitTimestamp.Nanosecond() != 0 {
			applyingTransactionOriginalCommit = float64(applyingTransactionOriginalCommitTimestamp.Unix())
		} else {
			applyingTransactionOriginalCommit = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitTimestampDesc, prometheus.GaugeValue, applyingTransactionOriginalCommit,
			channelName, workerId,
		)

		if applyingTransactionImmediateCommitTimestamp.Nanosecond() != 0 {
			applyingTransactionImmediateCommit = float64(applyingTransactionImmediateCommitTimestamp.Unix())
		} else {
			applyingTransactionImmediateCommit = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitTimestampDesc, prometheus.GaugeValue, applyingTransactionImmediateCommit,
			channelName, workerId,
		)

		if applyingTransactionStartApplyTimestamp.Nanosecond() != 0 {
			applyingTransactionStartApply = float64(applyingTransactionStartApplyTimestamp.Unix())
		} else {
			applyingTransactionStartApply = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplyTimestampDesc, prometheus.GaugeValue, applyingTransactionStartApply,
			channelName, workerId,
		)

		if applyingTransactionOriginalCommitTimestamp.Nanosecond() != 0 {
			tempLag, convErr := strconv.ParseFloat(string(replicationLag), 64)
			if convErr != nil {
				lag = tempLag
			} else {
				lag = -1
			}
		} else {
			lag = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerReplicaionLagDesc, prometheus.GaugeValue, lag,
			channelName, workerId,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationApplierStatsByWorker{}
