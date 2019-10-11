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

// Scrape `performance_schema.file_summary_by_event_name`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const perfFileEventsQuery = `
	SELECT
	    EVENT_NAME,
	    COUNT_READ, SUM_TIMER_READ, SUM_NUMBER_OF_BYTES_READ,
	    COUNT_WRITE, SUM_TIMER_WRITE, SUM_NUMBER_OF_BYTES_WRITE,
	    COUNT_MISC, SUM_TIMER_MISC
	  FROM performance_schema.file_summary_by_event_name
	`

// Metric descriptors.
var (
	performanceSchemaFileEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_total"),
		"The total file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
	performanceSchemaFileEventsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_seconds_total"),
		"The total seconds of file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
	performanceSchemaFileEventsBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_bytes_total"),
		"The total bytes of file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
)

// ScrapePerfFileEvents collects from `performance_schema.file_summary_by_event_name`.
type ScrapePerfFileEvents struct{}

// Name of the Scraper. Should be unique.
func (ScrapePerfFileEvents) Name() string {
	return "perf_schema.file_events"
}

// Help describes the role of the Scraper.
func (ScrapePerfFileEvents) Help() string {
	return "Collect metrics from performance_schema.file_summary_by_event_name"
}

// Version of MySQL from which scraper is available.
func (ScrapePerfFileEvents) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfFileEvents) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	// Timers here are returned in picoseconds.
	perfSchemaFileEventsRows, err := db.QueryContext(ctx, perfFileEventsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaFileEventsRows.Close()

	var (
		eventName                         string
		countRead, timeRead, bytesRead    uint64
		countWrite, timeWrite, bytesWrite uint64
		countMisc, timeMisc               uint64
	)
	for perfSchemaFileEventsRows.Next() {
		if err := perfSchemaFileEventsRows.Scan(
			&eventName,
			&countRead, &timeRead, &bytesRead,
			&countWrite, &timeWrite, &bytesWrite,
			&countMisc, &timeMisc,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countRead),
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeRead)/picoSeconds,
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsBytesDesc, prometheus.CounterValue, float64(bytesRead),
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countWrite),
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeWrite)/picoSeconds,
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsBytesDesc, prometheus.CounterValue, float64(bytesWrite),
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countMisc),
			eventName, "misc",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeMisc)/picoSeconds,
			eventName, "misc",
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapePerfFileEvents{}
