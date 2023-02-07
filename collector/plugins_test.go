package collector

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePlugins(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	columns := []string{"Name", "Status", "Type", "Library", "License"}
	rows := sqlmock.NewRows(columns).
		AddRow("INNODB_SYS_COLUMNS", "ACTIVE", "INFORMATION SCHEMA", nil, "GPL").
		AddRow("MRG_MYISAM", "ACTIVE", "STORAGE ENGINE", nil, "GPL").
		AddRow(nil, nil, nil, nil, nil)
	mock.ExpectQuery(sanitizeQuery(pluginsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePlugins{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()
	counterExpected := []MetricResult{
		{labels: labelMap{"name": "INNODB_SYS_COLUMNS", "status": "ACTIVE", "type": "INFORMATION SCHEMA", "library": "", "license": "GPL"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"name": "MRG_MYISAM", "status": "ACTIVE", "type": "STORAGE ENGINE", "library": "", "license": "GPL"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"name": "", "status": "", "type": "", "library": "", "license": ""}, value: 1, metricType: dto.MetricType_GAUGE},
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
