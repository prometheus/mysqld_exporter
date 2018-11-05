package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeInfoSchemaInnodbTablespaces(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"SPACE", "NAME", "FILE_FORMAT", "ROW_FORMAT", "SPACE_TYPE", "FILE_SIZE", "ALLOCATED_SIZE"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "sys/sys_config", "Barracuda", "Dynamic", "Single", 100, 100).
		AddRow(2, "db/compressed", "Barracuda", "Compressed", "Single", 300, 200)
	mock.ExpectQuery(sanitizeQuery(innodbTablespacesQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeInfoSchemaInnodbTablespaces{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"tablespace_name": "sys/sys_config", "file_format": "Barracuda", "row_format": "Dynamic", "space_type": "Single"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "sys/sys_config"}, value: 100, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "sys/sys_config"}, value: 100, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed", "file_format": "Barracuda", "row_format": "Compressed", "space_type": "Single"}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed"}, value: 300, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed"}, value: 200, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
