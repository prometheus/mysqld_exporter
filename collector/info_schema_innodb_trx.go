// Copyright The Prometheus Authors
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

// Scrape `information_schema.innodb_trx`.

package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const innodbTrxQuery = `
	SELECT
		trx_state,
		COUNT(*) AS transactions,
		COALESCE(MAX(TIMESTAMPDIFF(SECOND, trx_started, NOW())), 0) AS oldest_transaction_seconds
	FROM information_schema.innodb_trx
	GROUP BY trx_state
`

var (
	innodbTrxTransactionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_trx_transactions"),
		"Number of active InnoDB transactions by state.",
		[]string{"state"}, nil,
	)
	innodbTrxOldestTransactionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_trx_oldest_transaction_seconds"),
		"Age in seconds of the oldest active InnoDB transaction by state.",
		[]string{"state"}, nil,
	)
)

// ScrapeInnodbTrx collects from information_schema.innodb_trx.
type ScrapeInnodbTrx struct{}

func (ScrapeInnodbTrx) Name() string {
	return informationSchema + ".innodb_trx"
}

func (ScrapeInnodbTrx) Help() string {
	return "Collect transaction counts and oldest transaction age from information_schema.innodb_trx"
}

func (ScrapeInnodbTrx) Version() float64 {
	return 5.5
}

func (ScrapeInnodbTrx) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) (err error) {
	rows, err := instance.getDB().QueryContext(ctx, innodbTrxQuery)
	if err != nil {
		return fmt.Errorf("query InnoDB transactions: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
		var state string
		var count, oldestAge float64
		if err := rows.Scan(&state, &count, &oldestAge); err != nil {
			return fmt.Errorf("scan InnoDB transactions: %w", err)
		}
		ch <- prometheus.MustNewConstMetric(innodbTrxTransactionsDesc, prometheus.GaugeValue, count, state)
		ch <- prometheus.MustNewConstMetric(innodbTrxOldestTransactionDesc, prometheus.GaugeValue, oldestAge, state)
	}
	return rows.Err()
}

var _ Scraper = ScrapeInnodbTrx{}
