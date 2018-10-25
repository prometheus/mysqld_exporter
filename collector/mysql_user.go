// Scrape `mysql.user`.

package collector

import (
	"database/sql"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const mysqlUserQuery = `
		  SELECT
		    user,
		    host,
		    (
		      CASE WHEN (max_user_connections = 0) THEN
		        (
		          CASE WHEN (@@global.max_user_connections = 0) THEN
			    @@global.max_connections
			  ELSE
			    @@global.max_user_connections
			  END
			)
		      ELSE
		        max_user_connections
		      END
		    ) AS max_user_connections
		  FROM mysql.user
		`

// Metric descriptors.
var (
	userDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, mysql, "max_user_connections"),
		"The number of max_user_connections by user.",
		[]string{"src_user", "src_host"}, nil)
)

// ScrapeUser collects from `information_schema.processlist`.
type ScrapeUser struct{}

// Name of the Scraper. Should be unique.
func (ScrapeUser) Name() string {
	return mysql + ".user"
}

// Help describes the role of the Scraper.
func (ScrapeUser) Help() string {
	return "Collect data from mysql.user"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeUser) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	userQuery := fmt.Sprintf(
		mysqlUserQuery,
	)
	userRows, err := db.Query(userQuery)
	if err != nil {
		return err
	}
	defer userRows.Close()

	var (
		user                 string
		host                 string
		max_user_connections uint32
	)

	for userRows.Next() {
		err = userRows.Scan(&user, &host, &max_user_connections)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(userDesc, prometheus.GaugeValue, float64(max_user_connections), user, host)
	}

	return nil
}
