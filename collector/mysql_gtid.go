// Copyright 2025 The Prometheus Authors
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

// Scrape a transaction counter from the gtid_executed

package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sjmudd/mysqlgtid"
)

const (
	// transactions is the Metric subsystem we use.
	prometheusSubsystem = "gtid"
	prometheusName      = "transactions"
	// gtidTransactionCountQuery is the query used to fetch gtid_executed.
	// With this value we can convert it to an incremental transaction counter.
	gtidTransactionCountQuery = "SELECT @@gtid_executed"
)

var (
	// Metric descriptors.
	GtidTransactionCounterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, prometheusSubsystem, prometheusName),
		"Number of GTID transactions",
		[]string{}, nil,
	)
)

// ScrapeGtidExecuted scrapes transaction count from @@gtid_executed.
type ScrapeGtidExecuted struct{}

// Name of the Scraper. Should be unique.
func (ScrapeGtidExecuted) Name() string {
	return "gtid_executed_transactions_scraper"
}

// Help describes the role of the Scraper.
func (ScrapeGtidExecuted) Help() string {
	return "Number of GTID transactions"
}

// Version of MySQL from which scraper is available.
func (ScrapeGtidExecuted) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeGtidExecuted) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	var gtidExecuted string

	db := instance.getDB()
	if err := db.QueryRowContext(ctx, gtidTransactionCountQuery).Scan(&gtidExecuted); err != nil {
		return err
	}

	// convert into a counter
	gtidExecutedTransactionCounterIntVal, err := mysqlgtid.TransactionCount(gtidExecuted)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		GtidTransactionCounterDesc,
		prometheus.CounterValue,
		float64(gtidExecutedTransactionCounterIntVal),
	)

	return nil
}

// check interface
var _ Scraper = ScrapeGtidExecuted{}
