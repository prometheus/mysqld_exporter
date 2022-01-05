// Copyright 2020 The Prometheus Authors
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

// Scrape `performance_schema.memory_summary_global_by_event_name`.

package collector

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/alecthomas/kingpin.v2"
)

const perfMemoryEventsQuery = `
	SELECT
		EVENT_NAME, SUM_NUMBER_OF_BYTES_ALLOC, SUM_NUMBER_OF_BYTES_FREE,
		CURRENT_NUMBER_OF_BYTES_USED
	FROM performance_schema.memory_summary_global_by_event_name
		where COUNT_ALLOC > 0;
`

// Tunable flags.
var (
	performanceSchemaMemoryEventsRemovePrefix = kingpin.Flag(
		"collect.perf_schema.memory_events.remove_prefix",
		"Remove instrument prefix in performance_schema.memory_summary_global_by_event_name",
	).Default("memory/").String()
)

// Metric descriptors.
var (
	performanceSchemaMemoryBytesAllocDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "memory_events_alloc_bytes_total"),
		"The total number of bytes allocated by events.",
		[]string{"event_name"}, nil,
	)
	performanceSchemaMemoryBytesFreeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "memory_events_free_bytes_total"),
		"The total number of bytes freed by events.",
		[]string{"event_name"}, nil,
	)
	perforanceSchemaMemoryUsedBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "memory_events_used_bytes"),
		"The number of bytes currently allocated by events.",
		[]string{"event_name"}, nil,
	)
)

// ScrapePerfMemoryEvents collects from `performance_schema.memory_summary_global_by_event_name`.
type ScrapePerfMemoryEvents struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfMemoryEvents) Name() string {
	return "perf_schema.memory_events"
}

// Help describes the role of the Scraper.
func (ScrapePerfMemoryEvents) Help() string {
	return "Collect metrics from performance_schema.memory_summary_global_by_event_name"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfMemoryEvents) Version() float64 {
	return 5.7
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfMemoryEvents) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	perfSchemaMemoryEventsRows, err := db.QueryContext(ctx, perfMemoryEventsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaMemoryEventsRows.Close()

	var (
		eventName    string
		bytesAlloc   uint64
		bytesFree    uint64
		currentBytes int64
	)

	for perfSchemaMemoryEventsRows.Next() {
		if err := perfSchemaMemoryEventsRows.Scan(
			&eventName, &bytesAlloc, &bytesFree, &currentBytes,
		); err != nil {
			return err
		}

		eventName := strings.TrimPrefix(eventName, *performanceSchemaMemoryEventsRemovePrefix)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaMemoryBytesAllocDesc, prometheus.CounterValue, float64(bytesAlloc), eventName,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaMemoryBytesFreeDesc, prometheus.CounterValue, float64(bytesFree), eventName,
		)
		ch <- prometheus.MustNewConstMetric(
			perforanceSchemaMemoryUsedBytesDesc, prometheus.GaugeValue, float64(currentBytes), eventName,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfMemoryEvents{}
