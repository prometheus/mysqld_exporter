package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeSlaveStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Master_Host", "Read_Master_Log_Pos", "Slave_IO_Running", "Slave_SQL_Running", "Seconds_Behind_Master"}
	rows := sqlmock.NewRows(columns).
		AddRow("127.0.0.1", "1", "Connecting", "Yes", "2")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE STATUS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveStatus{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 0, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 2, metricType: dto.MetricType_UNTYPED},
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
