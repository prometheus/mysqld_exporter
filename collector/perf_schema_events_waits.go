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

// Scrape `performance_schema.events_waits_summary_global_by_event_name`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfEventsWaitsQuery = `
	SELECT EVENT_NAME, COUNT_STAR, SUM_TIMER_WAIT
	  FROM performance_schema.events_waits_summary_global_by_event_name
	`

// Metric descriptors.
var (
	performanceSchemaEventsWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_waits_total"),
		"The total events waits by event name.",
		[]string{"event_name"}, nil,
	)
	performanceSchemaEventsWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_waits_seconds_total"),
		"The total seconds of events waits by event name.",
		[]string{"event_name"}, nil,
	)
)

// ScrapePerfEventsWaits collects from `performance_schema.events_waits_summary_global_by_event_name`.
type ScrapePerfEventsWaits struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfEventsWaits) Name() string {
	return "perf_schema.eventswaits"
}

// Help describes the role of the Scraper.
func (ScrapePerfEventsWaits) Help() string {
	return "Collect metrics from performance_schema.events_waits_summary_global_by_event_name"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfEventsWaits) Version() float64 {
	return 5.5
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfEventsWaits) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	// Timers here are returned in picoseconds.
	perfSchemaEventsWaitsRows, err := db.QueryContext(ctx, perfEventsWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaEventsWaitsRows.Close()

	var (
		eventName   string
		count, time uint64
	)

	for perfSchemaEventsWaitsRows.Next() {
		if err := perfSchemaEventsWaitsRows.Scan(
			&eventName, &count, &time,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsWaitsDesc, prometheus.CounterValue, float64(count),
			eventName,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsWaitsTimeDesc, prometheus.CounterValue, float64(time)/picoSeconds,
			eventName,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfEventsWaits{}
