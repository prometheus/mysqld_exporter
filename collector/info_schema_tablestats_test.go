package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeTableStat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(sanitizeQuery(userstatCheckQuery)).WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("userstat", "ON"))

	columns := []string{"TABLE_SCHEMA", "TABLE_NAME", "ROWS_READ", "ROWS_CHANGED", "ROWS_CHANGED_X_INDEXES"}
	rows := sqlmock.NewRows(columns).
		AddRow("mysql", "db", 238, 0, 8).
		AddRow("mysql", "proxies_priv", 99, 1, 0).
		AddRow("mysql", "user", 1064, 2, 5)
	mock.ExpectQuery(sanitizeQuery(tableStatQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeTableStat{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"schema": "mysql", "table": "db"}, value: 238},
		{labels: labelMap{"schema": "mysql", "table": "db"}, value: 0},
		{labels: labelMap{"schema": "mysql", "table": "db"}, value: 8},
		{labels: labelMap{"schema": "mysql", "table": "proxies_priv"}, value: 99},
		{labels: labelMap{"schema": "mysql", "table": "proxies_priv"}, value: 1},
		{labels: labelMap{"schema": "mysql", "table": "proxies_priv"}, value: 0},
		{labels: labelMap{"schema": "mysql", "table": "user"}, value: 1064},
		{labels: labelMap{"schema": "mysql", "table": "user"}, value: 2},
		{labels: labelMap{"schema": "mysql", "table": "user"}, value: 5},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
