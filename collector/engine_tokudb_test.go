package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestSanitizeTokudbMetric(t *testing.T) {
	samples := map[string]string{
		"loader: number of calls to loader->close() that failed": "loader_number_of_calls_to_loader_close_that_failed",
		"ft: promotion: stopped anyway, after locking the child": "ft_promotion_stopped_anyway_after_locking_the_child",
		"ft: basement nodes deserialized with fixed-keysize":     "ft_basement_nodes_deserialized_with_fixed_keysize",
		"memory: number of bytes used (requested + overhead)":    "memory_number_of_bytes_used_requested_and_overhead",
		"ft: uncompressed / compressed bytes written (overall)":  "ft_uncompressed_and_compressed_bytes_written_overall",
	}
	convey.Convey("Replacement tests", t, func() {
		for metric := range samples {
			got := sanitizeTokudbMetric(metric)
			convey.So(got, convey.ShouldEqual, samples[metric])
		}
	})
}

func TestScrapeEngineTokudbStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Type", "Name", "Status"}
	rows := sqlmock.NewRows(columns).
		AddRow("TokuDB", "indexer: number of calls to indexer->build() succeeded", "1").
		AddRow("TokuDB", "ft: promotion: stopped anyway, after locking the child", "45316247").
		AddRow("TokuDB", "memory: mallocator version", "3.3.1-0-g9ef9d9e8c271cdf14f664b871a8f98c827714784").
		AddRow("TokuDB", "filesystem: most recent disk full", "Thu Jan  1 00:00:00 1970").
		AddRow("TokuDB", "locktree: time spent ending the STO early (seconds)", "9115.904484")

	mock.ExpectQuery(sanitizeQuery(engineTokudbStatusQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeEngineTokudbStatus{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricsExpected := []MetricResult{
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 45316247, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 9115.904484, metricType: dto.MetricType_UNTYPED},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricsExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
