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
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_original_commit_second"),
		"A timestamp shows when the last transaction applied by this worker was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_immediate_commit_second"),
		"A timestamp shows when the last transaction applied by this worker was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_start_apply_second"),
		"A timestamp shows when this worker started applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "last_applied_transaction_end_apply_second"),
		"A shows when this worker finished applying the last applied transaction.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_original_commit_second"),
		"A timestamp that shows when the transaction this worker is currently applying was committed on the original master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitSecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_immediate_commit_second"),
		"A timestamp shows when the transaction this worker is currently applying was committed on the immediate master.",
		[]string{"channel_name", "member_id"}, nil,
	)

	performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplySecondDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "applying_transaction_start_apply_second"),
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
		channelName, workerId                                                                   string
		lastAppliedTransactionOriginalCommitTime, lastAppliedTransactionImmediateCommitTime     time.Time
		lastAppliedTransactionStartApplyTime, lastAppliedTransactionEndApplyTime                time.Time
		applyingTransactionOriginalCommitTime, applyingTransactionImmediateCommitTime           time.Time
		applyingTransactionStartApplyTime                                                       time.Time
		lastAppliedTransactionOriginalCommitSecond, lastAppliedTransactionImmediateCommitSecond float64
		lastAppliedTransactionStartApplySecond, lastAppliedTransactionEndApplySecond            float64
		applyingTransactionOriginalCommitSecond, applyingTransactionImmediateCommitSecond       float64
		applyingTransactionStartApplySecond                                                     float64
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
			lastAppliedTransactionOriginalCommitSecond = float64(lastAppliedTransactionOriginalCommitTime.Unix())
		} else {
			lastAppliedTransactionOriginalCommitSecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionOriginalCommitSecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionOriginalCommitSecond, channelName, workerId,
		)

		if lastAppliedTransactionImmediateCommitTime.Nanosecond() != 0 {
			lastAppliedTransactionImmediateCommitSecond = float64(lastAppliedTransactionImmediateCommitTime.Unix())
		} else {
			lastAppliedTransactionImmediateCommitSecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionImmediateCommitSecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionImmediateCommitSecond, channelName, workerId,
		)

		if lastAppliedTransactionStartApplyTime.Nanosecond() != 0 {
			lastAppliedTransactionStartApplySecond = float64(lastAppliedTransactionStartApplyTime.Unix())
		} else {
			lastAppliedTransactionStartApplySecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionStartApplySecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionStartApplySecond, channelName, workerId,
		)

		if lastAppliedTransactionEndApplyTime.Nanosecond() != 0 {
			lastAppliedTransactionEndApplySecond = float64(lastAppliedTransactionEndApplyTime.Unix())
		} else {
			lastAppliedTransactionEndApplySecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerLastAppliedTransactionEndApplySecondDesc,
			prometheus.GaugeValue, lastAppliedTransactionEndApplySecond, channelName, workerId,
		)

		if applyingTransactionOriginalCommitTime.Nanosecond() != 0 {
			applyingTransactionOriginalCommitSecond = float64(applyingTransactionOriginalCommitTime.Unix())
		} else {
			applyingTransactionOriginalCommitSecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionOriginalCommitSecondDesc,
			prometheus.GaugeValue, applyingTransactionOriginalCommitSecond, channelName, workerId,
		)

		if applyingTransactionImmediateCommitTime.Nanosecond() != 0 {
			applyingTransactionImmediateCommitSecond = float64(applyingTransactionImmediateCommitTime.Unix())
		} else {
			applyingTransactionImmediateCommitSecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionImmediateCommitSecondDesc,
			prometheus.GaugeValue, applyingTransactionImmediateCommitSecond, channelName, workerId,
		)

		if applyingTransactionStartApplyTime.Nanosecond() != 0 {
			applyingTransactionStartApplySecond = float64(applyingTransactionStartApplyTime.Unix())
		} else {
			applyingTransactionStartApplySecond = 0
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaReplicationApplierStatsByWorkerApplyingTransactionStartApplySecondDesc,
			prometheus.GaugeValue, applyingTransactionStartApplySecond, channelName, workerId,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfReplicationApplierStatsByWorker{}
