package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeInnodbCmpMem(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"page_size", "buffer", "pages_used", "pages_free", "relocation_ops", "relocation_time"}
	rows := sqlmock.NewRows(columns).
		AddRow("1024", "0", 30, 40, 50, 60)
	mock.ExpectQuery(sanitizeQuery(innodbCmpMemQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeInnodbCmpMem{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"page_size": "1024", "buffer": "0"}, value: 30, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer": "0"}, value: 40, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer": "0"}, value: 50, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer": "0"}, value: 0.06, metricType: dto.MetricType_COUNTER},
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
