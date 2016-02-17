package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func readCounter(m prometheus.Metric) (string, float64) {
	pb := &dto.Metric{}
	m.Write(pb)
	label := pb.Label[0].GetValue()
	value := pb.GetCounter().GetValue()
	return label, value
}

func sanitizeQuery(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

func Test_scrapeQueryResponseTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error when opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(queryResponseCheckQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

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
		AddRow(1000000.000000, 0, 0.000000)
	mock.ExpectQuery(sanitizeQuery(queryResponseTimeQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeQueryResponseTime(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	// Test counters one by one to easy spot a mismatch
	expectCounts := map[string]float64{
		"1e-06":  0,
		"1e-05":  0.000797,
		"0.0001": 0.108118,
		"0.001":  0.443513,
		"0.01":   0.9657769999999999,
		"0.1":    1.3099859999999999,
		"1":      1.5773549999999998,
		"10":     1.5773549999999998,
		"100":    1.5773549999999998,
		"1000":   1.5773549999999998,
		"10000":  1.5773549999999998,
		"100000": 1.5773549999999998,
		"1e+06":  1.5773549999999998,
		"+Inf":   1.5773549999999998,
	}
	for _ = range expectCounts {
		if label, got := readCounter((<-ch).(prometheus.Metric)); expectCounts[label] != got {
			t.Errorf("Counter: [%s] expected %v, got %v", label, expectCounts[label], got)
		}
	}

	// Test histogram
	expectTimes := map[float64]uint64{
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
	expectHistogram := prometheus.MustNewConstHistogram(infoSchemaQueryResponseTimeCountDesc,
		4528, 1.5773549999999998, expectTimes)
	expectPb := &dto.Metric{}
	expectHistogram.Write(expectPb)

	// Read the last item from channel
	gotPb := &dto.Metric{}
	gotHistogram := <-ch
	gotHistogram.Write(gotPb)
	if !reflect.DeepEqual(expectPb.Histogram, gotPb.Histogram) {
		t.Errorf("Histogram: expected %+v, got %+v", expectPb.Histogram, gotPb.Histogram)
	}

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
