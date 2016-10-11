package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeQueryResponseTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(queryResponseCheckQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(1))

	rows := sqlmock.NewRows([]string{"TIME", "COUNT", "TOTAL"}).
		AddRow(0.000001, 124, 0.000000).
		AddRow(0.000010, 179, 0.000797).
		AddRow(0.000100, 2859, 0.107321).
		AddRow(0.001000, 1085, 0.335395).
		AddRow(0.010000, 269, 0.522264).
		AddRow(0.100000, 11, 0.344209).
		AddRow(1.000000, 1, 0.267369).
		AddRow(10.000000, 0, 0.000000).
		AddRow(100.000000, 0, 0.000000).
		AddRow(1000.000000, 0, 0.000000).
		AddRow(10000.000000, 0, 0.000000).
		AddRow(100000.000000, 0, 0.000000).
		AddRow(1000000.000000, 0, 0.000000).
		AddRow("TOO LONG", 0, "TOO LONG")
	mock.ExpectQuery(sanitizeQuery(queryResponseTimeQueries[0])).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = ScrapeQueryResponseTime(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	// Test counters
	expectTimes := []MetricResult{
		{labels: labelMap{"le": "1e-06"}, value: 0},
		{labels: labelMap{"le": "1e-05"}, value: 0.000797},
		{labels: labelMap{"le": "0.0001"}, value: 0.108118},
		{labels: labelMap{"le": "0.001"}, value: 0.443513},
		{labels: labelMap{"le": "0.01"}, value: 0.9657769999999999},
		{labels: labelMap{"le": "0.1"}, value: 1.3099859999999999},
		{labels: labelMap{"le": "1"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "10"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "100"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "1000"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "10000"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "100000"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "1e+06"}, value: 1.5773549999999998},
		{labels: labelMap{"le": "+Inf"}, value: 1.5773549999999998},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expectTimes {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Test histogram
	expectCounts := map[float64]uint64{
		1e-06:  124,
		1e-05:  303,
		0.0001: 3162,
		0.001:  4247,
		0.01:   4516,
		0.1:    4527,
		1:      4528,
		10:     4528,
		100:    4528,
		1000:   4528,
		10000:  4528,
		100000: 4528,
		1e+06:  4528,
	}
	expectHistogram := prometheus.MustNewConstHistogram(infoSchemaQueryResponseTimeCountDescs[0],
		4528, 1.5773549999999998, expectCounts)
	expectPb := &dto.Metric{}
	expectHistogram.Write(expectPb)

	gotPb := &dto.Metric{}
	gotHistogram := <-ch // read the last item from channel
	gotHistogram.Write(gotPb)
	convey.Convey("Histogram comparison", t, func() {
		convey.So(expectPb.Histogram, convey.ShouldResemble, gotPb.Histogram)
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
