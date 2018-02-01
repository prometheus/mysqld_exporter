package collector

import (
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

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfReplicationGroupMemberStats) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	perfReplicationGroupMemeberStatsRows, err := db.Query(perfReplicationGroupMemeberStatsQuery)
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
