// Scrape `performance_schema.events_waits_summary_global_by_event_name`.

package collector

import (
	"database/sql"

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

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapePerfEventsWaits) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaEventsWaitsRows, err := db.Query(perfEventsWaitsQuery)
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
