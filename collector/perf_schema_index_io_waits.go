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

// Scrape `performance_schema.table_io_waits_summary_by_index_usage`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfIndexIOWaitsQuery = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME,
	    COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE,
	    SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
	  FROM performance_schema.table_io_waits_summary_by_index_usage
	  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
	`

// Metric descriptors.
var (
	performanceSchemaIndexWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "index_io_waits_total"),
		"The total number of index I/O wait events for each index and operation.",
		[]string{"schema", "name", "index", "operation"}, nil,
	)
	performanceSchemaIndexWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "index_io_waits_seconds_total"),
		"The total time of index I/O wait events for each index and operation.",
		[]string{"schema", "name", "index", "operation"}, nil,
	)
)

// ScrapePerfIndexIOWaits collects for `performance_schema.table_io_waits_summary_by_index_usage`.
type ScrapePerfIndexIOWaits struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfIndexIOWaits) Name() string {
	return "perf_schema.indexiowaits"
}

// Help describes the role of the Scraper.
func (ScrapePerfIndexIOWaits) Help() string {
	return "Collect metrics from performance_schema.table_io_waits_summary_by_index_usage"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfIndexIOWaits) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfIndexIOWaits) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	perfSchemaIndexWaitsRows, err := db.QueryContext(ctx, perfIndexIOWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaIndexWaitsRows.Close()

	var (
		objectSchema, objectName, indexName               string
		countFetch, countInsert, countUpdate, countDelete uint64
		timeFetch, timeInsert, timeUpdate, timeDelete     uint64
	)

	for perfSchemaIndexWaitsRows.Next() {
		if err := perfSchemaIndexWaitsRows.Scan(
			&objectSchema, &objectName, &indexName,
			&countFetch, &countInsert, &countUpdate, &countDelete,
			&timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countFetch),
			objectSchema, objectName, indexName, "fetch",
		)
		// We only include the insert column when indexName is NONE.
		if indexName == "NONE" {
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countInsert),
				objectSchema, objectName, indexName, "insert",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countUpdate),
			objectSchema, objectName, indexName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countDelete),
			objectSchema, objectName, indexName, "delete",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeFetch)/picoSeconds,
			objectSchema, objectName, indexName, "fetch",
		)
		// We only update write columns when indexName is NONE.
		if indexName == "NONE" {
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeInsert)/picoSeconds,
				objectSchema, objectName, indexName, "insert",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeUpdate)/picoSeconds,
			objectSchema, objectName, indexName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeDelete)/picoSeconds,
			objectSchema, objectName, indexName, "delete",
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfIndexIOWaits{}
