package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestScrapeHeartbeat(t *testing.T) {
	_, err := kingpin.CommandLine.Parse([]string{
		"--collect.heartbeat.database", "heartbeat-test",
		"--collect.heartbeat.table", "heartbeat-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"UNIX_TIMESTAMP(ts)", "UNIX_TIMESTAMP(NOW(6))", "server_id"}
	rows := sqlmock.NewRows(columns).
		AddRow("1487597613.001320", "1487598113.448042", 1)
	mock.ExpectQuery(sanitizeQuery("SELECT UNIX_TIMESTAMP(ts), UNIX_TIMESTAMP(NOW(6)), server_id from `heartbeat-test`.`heartbeat-test`")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeHeartbeat{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"server_id": "1"}, value: 1487598113.448042, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "1"}, value: 1487597613.00132, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
