package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeBinlogSize(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(logbinQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(1))

	columns := []string{"Log_name", "File_size"}
	rows := sqlmock.NewRows(columns).
		AddRow("centos6-bin.000001", "1813").
		AddRow("centos6-bin.000002", "120").
		AddRow("centos6-bin.000444", "573009")
	mock.ExpectQuery(sanitizeQuery(binlogQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeBinlogSize{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{}, value: 574942, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 3, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 444, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
