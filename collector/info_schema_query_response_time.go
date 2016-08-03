// Scrape `information_schema.query_response_time`.

package collector

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	queryResponseCheckQuery = `SELECT @@query_response_time_stats`
	queryResponseTimeQuery  = `
		SELECT
		    TIME, COUNT, TOTAL
		  FROM information_schema.query_response_time
		`
)

var (
	infoSchemaQueryResponseTimeCountDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "query_response_time_count"),
		"The number of queries according to the length of time they took to execute.",
		[]string{}, nil,
	)
	infoSchemaQueryResponseTimeTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "query_response_time_total"),
		"Total time of queries according to the length of time they took to execute separately.",
		[]string{"le"}, nil,
	)
)

// ScrapeQueryResponseTime collects from `information_schema.query_response_time`.
func ScrapeQueryResponseTime(db *sql.DB, ch chan<- prometheus.Metric) error {
	var queryStats uint8
	err := db.QueryRow(queryResponseCheckQuery).Scan(&queryStats)
	if err != nil {
		log.Debugln("Query response time distribution is not present.")
		return nil
	}
	if queryStats == 0 {
		log.Debugln("MySQL @@query_response_time_stats is OFF.")
		return nil
	}

	queryDistributionRows, err := db.Query(queryResponseTimeQuery)
	if err != nil {
		return err
	}
	defer queryDistributionRows.Close()

	var (
		length       string
		count        uint64
		total        string
		histogramCnt uint64
		histogramSum float64
		countBuckets = map[float64]uint64{}
	)

	for queryDistributionRows.Next() {
		err = queryDistributionRows.Scan(
			&length,
			&count,
			&total,
		)
		if err != nil {
			return err
		}

		length, _ := strconv.ParseFloat(strings.TrimSpace(length), 64)
		total, _ := strconv.ParseFloat(strings.TrimSpace(total), 64)
		histogramCnt += count
		histogramSum += total
		// Special case for "TOO LONG" row where we take into account the count field which is the only available
		// and do not add it as a part of histogram or metric
		if length == 0 {
			continue
		}
		countBuckets[length] = histogramCnt
		// No histogram with query total times because they are float
		ch <- prometheus.MustNewConstMetric(
			infoSchemaQueryResponseTimeTotalDesc, prometheus.CounterValue, histogramSum,
			fmt.Sprintf("%v", length),
		)
	}
	ch <- prometheus.MustNewConstMetric(
		infoSchemaQueryResponseTimeTotalDesc, prometheus.CounterValue, histogramSum,
		"+Inf",
	)
	// Create histogram with query counts
	ch <- prometheus.MustNewConstHistogram(
		infoSchemaQueryResponseTimeCountDesc, histogramCnt, histogramSum, countBuckets,
	)
	return nil
}
