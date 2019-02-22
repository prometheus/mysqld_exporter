// Copyright 2019 The Prometheus Authors
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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const perfReplicationApplierStatsByWorkerQuery = `
	SELECT 
	    CHANNEL_NAME,
		WORKER_ID,
		LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP,
		LAST_APPLIED_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP,
		LAST_APPLIED_TRANSACTION_START_APPLY_TIMESTAMP,
		LAST_APPLIED_TRANSACTION_END_APPLY_TIMESTAMP,
		APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP,
		APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP, 
	  	APPLYING_TRANSACTION_START_APPLY_TIMESTAMP
    FROM performance_schema.replication_applier_status_by_worker
	`

// Metric descriptors.
var (
	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionOriginalCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_original_commit_timestamp_seconds"),
		"A timestamp shows when the last transaction applied by this worker was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_immediate_commit_timestamp_seconds"),
		"A timestamp shows when the last transaction applied by this worker was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_start_apply_timestamp_seconds"),
		"A timestamp shows when this worker started applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_end_apply_timestamp_seconds"),
		"A shows when this worker finished applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_original_commit_timestamp_seconds"),
		"A timestamp that shows when the transaction this worker is currently applying was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_immediate_commit_timestamp_seconds"),
		"A timestamp shows when the transaction this worker is currently applying was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_start_apply_timestamp_seconds"),
		"A timestamp shows when this worker started its first attempt to apply the transaction that is currently being applied.",
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
		channelName, workerId                                                                     string
		lastAppliedTransactionOriginalCommitTime, lastAppliedTransactionImmediateCommitTime       time.Time
		lastAppliedTransactionStartApplyTime, lastAppliedTransactionEndApplyTime                  time.Time
		applyingTransactionOriginalCommitTime, applyingTransactionImmediateCommitTime             time.Time
		applyingTransactionStartApplyTime                                                         time.Time
		lastAppliedTransactionOriginalCommitSeconds, lastAppliedTransactionImmediateCommitSeconds float64
		lastAppliedTransactionStartApplySeconds, lastAppliedTransactionEndApplySeconds            float64
		applyingTransactionOriginalCommitSeconds, applyingTransactionImmediateCommitSeconds       float64
		applyingTransactionStartApplySeconds                                                      float64
	)

	for perfReplicationApplierStatsByWorkerRows.Next() {
		if err := perfReplicationApplierStatsByWorkerRows.Scan(
			&channelName, &workerId,
			&lastAppliedTransactionOriginalCommitTime, &lastAppliedTransactionImmediateCommitTime,
			&lastAppliedTransactionStartApplyTime, &lastAppliedTransactionEndApplyTime,
			&applyingTransactionOriginalCommitTime, &applyingTransactionImmediateCommitTime,
			&applyingTransactionStartApplyTime,
		); err != nil {
			return err
		}

		// Check if the value is 0, use a real 0
		if lastAppliedTransactionOriginalCommitTime.Nanosecond() != 0 {
			lastAppliedTransactionOriginalCommitSeconds = float64(lastAppliedTransactionOriginalCommitTime.UnixNano()) / 1e9
		} else {
			lastAppliedTransactionOriginalCommitSeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionOriginalCommitSecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionOriginalCommitSeconds, channelName, workerId,
		)

		if lastAppliedTransactionImmediateCommitTime.Nanosecond() != 0 {
			lastAppliedTransactionImmediateCommitSeconds = float64(lastAppliedTransactionImmediateCommitTime.UnixNano()) / 1e9
		} else {
			lastAppliedTransactionImmediateCommitSeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitSecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionImmediateCommitSeconds, channelName, workerId,
		)

		if lastAppliedTransactionStartApplyTime.Nanosecond() != 0 {
			lastAppliedTransactionStartApplySeconds = float64(lastAppliedTransactionStartApplyTime.UnixNano()) / 1e9
		} else {
			lastAppliedTransactionStartApplySeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplySecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionStartApplySeconds, channelName, workerId,
		)

		if lastAppliedTransactionEndApplyTime.Nanosecond() != 0 {
			lastAppliedTransactionEndApplySeconds = float64(lastAppliedTransactionEndApplyTime.UnixNano()) / 1e9
		} else {
			lastAppliedTransactionEndApplySeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplySecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionEndApplySeconds, channelName, workerId,
		)

		if applyingTransactionOriginalCommitTime.Nanosecond() != 0 {
			applyingTransactionOriginalCommitSeconds = float64(applyingTransactionOriginalCommitTime.UnixNano()) / 1e9
		} else {
			applyingTransactionOriginalCommitSeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitSecondDesc,
			prometheus.GaugeValue, applyingTransactionOriginalCommitSeconds, channelName, workerId,
		)

		if applyingTransactionImmediateCommitTime.Nanosecond() != 0 {
			applyingTransactionImmediateCommitSeconds = float64(applyingTransactionImmediateCommitTime.UnixNano()) / 1e9
		} else {
			applyingTransactionImmediateCommitSeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitSecondDesc,
			prometheus.GaugeValue, applyingTransactionImmediateCommitSeconds, channelName, workerId,
		)

		if applyingTransactionStartApplyTime.Nanosecond() != 0 {
			applyingTransactionStartApplySeconds = float64(applyingTransactionStartApplyTime.UnixNano()) / 1e9
		} else {
			applyingTransactionStartApplySeconds = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplySecondDesc,
			prometheus.GaugeValue, applyingTransactionStartApplySeconds, channelName, workerId,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationApplierStatsByWorker{}
