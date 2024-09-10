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

// Scrape `information_schema.innodb_trx`.

package collector

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaInnodbTrxLongQuery = `
		  SELECT (unix_timestamp(now()) - unix_timestamp(trx_started)) AS trx_seconds
			  FROM information_schema.innodb_trx tx
		   WHERE tx.trx_state IN ('RUNNING','LOCK WAIT','ROLLING BACK','COMMITTING')
  			 AND (unix_timestamp(now()) - unix_timestamp(tx.trx_started)) > %d
		   ORDER BY trx_seconds DESC
		   LIMIT 1
	`

// Tunable flags.
var (
	trxMinTime = kingpin.Flag(
		"collect.info_schema.innodb_trx.min_seconds",
		"Minimum time a thread must be in each state to be counted",
	).Default("0").Int()
)

// Metric descriptors.
var (
	trxTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_trx_long"),
		"Execution threshold time of long transactions",
		nil, nil)
)

// ScrapeInnodbTrxLong collects from `information_schema.innodb_trx`.
type ScrapeInnodbTrxLong struct{}

// Name of the Scraper. Should be unique.
func (ScrapeInnodbTrxLong) Name() string {
	return informationSchema + ".innodb_trx_long"
}

// Help describes the role of the Scraper.
func (ScrapeInnodbTrxLong) Help() string {
	return "Collect the currently longest transaction from the information_schema.innodb_trx"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbTrxLong) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeInnodbTrxLong) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger log.Logger) error {
	trxQuery := fmt.Sprintf(
		infoSchemaInnodbTrxLongQuery,
		*trxMinTime,
	)
	db := instance.getDB()
	innodbTrxLongRows, err := db.QueryContext(ctx, trxQuery)
	if err != nil {
		return err
	}
	defer innodbTrxLongRows.Close()

	var (
		trx_seconds uint32
		rowExists   bool
	)

	for innodbTrxLongRows.Next() {
		err = innodbTrxLongRows.Scan(&trx_seconds)
		if err != nil {
			return err
		}
		rowExists = true

		ch <- prometheus.MustNewConstMetric(trxTimeDesc, prometheus.GaugeValue, float64(trx_seconds))
	}

	if !rowExists {
		ch <- prometheus.MustNewConstMetric(trxTimeDesc, prometheus.GaugeValue, float64(0))
	}

	return nil
}

var _ Scraper = ScrapeInnodbTrxLong{}
