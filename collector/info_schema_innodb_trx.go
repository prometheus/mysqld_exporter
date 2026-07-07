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

// Scrape `information_schema.INNODB_TRX`.
package collector

import (
	"context"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaInnodbTRX = `
	SELECT count(1) FROM information_schema.INNODB_TRX WHERE trx_started < NOW() - INTERVAL %d MINUTE;
`

// Tunable flags.
var trxRunningTime = kingpin.Flag(
	"collector.info_schema_innodb_trx.running_time",
	"The running time in minutes for which to collect the number of running transactions.",
).Default("0").Int()

// Metric descriptors.
var (
	infoSchemaInnodbTrxDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_trx_running_transactions"),
		"Number of running transactions that have been running for more than the specified time.",
		nil, nil)
)

// ScrapeTransaction collects from `information_schema.INNODB_TRX`.
type ScrapeTransaction struct{}

// Name of the Scraper. Should be unique.
func (ScrapeTransaction) Name() string { return informationSchema + ".innodb_trx" }

// Help describes the role of the Scraper.
func (ScrapeTransaction) Help() string {
	return "Number of running transactions that have been running for more than the specified time."
}

// Version of MySQL from which scraper is available.
func (ScrapeTransaction) Version() float64 {
	return 5.7
}

func (ScrapeTransaction) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger log.Logger) error {
	query := fmt.Sprintf(infoSchemaInnodbTRX, *trxRunningTime)
	db := instance.getDB()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var trxRunning int

	for rows.Next() {
		if err := rows.Scan(&trxRunning); err != nil {
			return err
		}
	}
	ch <- prometheus.MustNewConstMetric(infoSchemaInnodbTrxDesc, prometheus.GaugeValue, float64(trxRunning))

	return nil
}

// check interface
var _ Scraper = ScrapeTransaction{}
