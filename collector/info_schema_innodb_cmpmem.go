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

// Scrape `information_schema.INNODB_CMPMEM`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const innodbCmpMemQuery = `
                SELECT
                  page_size, buffer_pool_instance, pages_used, pages_free, relocation_ops, relocation_time
                  FROM information_schema.innodb_cmpmem
                `

// Metric descriptors.
var (
	infoSchemaInnodbCmpMemPagesRead = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_used_total"),
		"Number of blocks of the size PAGE_SIZE that are currently in use.",
		[]string{"page_size", "buffer_pool"}, nil,
	)
	infoSchemaInnodbCmpMemPagesFree = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_pages_free_total"),
		"Number of blocks of the size PAGE_SIZE that are currently available for allocation.",
		[]string{"page_size", "buffer_pool"}, nil,
	)
	infoSchemaInnodbCmpMemRelocationOps = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_ops_total"),
		"Number of times a block of the size PAGE_SIZE has been relocated.",
		[]string{"page_size", "buffer_pool"}, nil,
	)
	infoSchemaInnodbCmpMemRelocationTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmpmem_relocation_time_seconds_total"),
		"Total time in seconds spent in relocating blocks.",
		[]string{"page_size", "buffer_pool"}, nil,
	)
)

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
type ScrapeInnodbCmpMem struct{}

// Name of the Scraper. Should be unique.
func (ScrapeInnodbCmpMem) Name() string {
	return informationSchema + ".innodb_cmpmem"
}

// Help describes the role of the Scraper.
func (ScrapeInnodbCmpMem) Help() string {
	return "Collect metrics from information_schema.innodb_cmpmem"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbCmpMem) Version() float64 {
	return 5.5
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeInnodbCmpMem) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	informationSchemaInnodbCmpMemRows, err := db.QueryContext(ctx, innodbCmpMemQuery)
	if err != nil {
		return err
	}
	defer informationSchemaInnodbCmpMemRows.Close()

	var (
		page_size, buffer_pool                                  string
		pages_used, pages_free, relocation_ops, relocation_time float64
	)

	for informationSchemaInnodbCmpMemRows.Next() {
		if err := informationSchemaInnodbCmpMemRows.Scan(
			&page_size, &buffer_pool, &pages_used, &pages_free, &relocation_ops, &relocation_time,
		); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemPagesRead, prometheus.CounterValue, pages_used, page_size, buffer_pool)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemPagesFree, prometheus.CounterValue, pages_free, page_size, buffer_pool)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemRelocationOps, prometheus.CounterValue, relocation_ops, page_size, buffer_pool)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpMemRelocationTime, prometheus.CounterValue, (relocation_time / 1000), page_size, buffer_pool)
	}
	return nil
}

// check interface
var _ Scraper = ScrapeInnodbCmpMem{}
