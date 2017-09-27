package collector

import (
	"testing"

	"flag"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapePerfFileInstances(t *testing.T) {
	err := flag.Set("collect.perf_schema.file_instances.filter", "")
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"FILE_NAME", "COUNT_READ", "COUNT_WRITE", "SUM_NUMBER_OF_BYTES_READ", "SUM_NUMBER_OF_BYTES_WRITE"}

	rows := sqlmock.NewRows(columns).
		AddRow("file_1", "3", "4", "725", "128").
		AddRow("file_2", "23", "12", "3123", "967")
	mock.ExpectQuery(sanitizeQuery(perfFileInstancesQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = ScrapePerfFileInstances(db, ch); err != nil {
			panic(fmt.Sprintf("error calling function on test: %s", err))
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"file_name": "file_1", "mode": "read"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_1", "mode": "write"}, value: 4, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_1", "mode": "read"}, value: 725, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_1", "mode": "write"}, value: 128, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_2", "mode": "read"}, value: 23, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_2", "mode": "write"}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_2", "mode": "read"}, value: 3123, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "file_2", "mode": "write"}, value: 967, metricType: dto.MetricType_COUNTER},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
