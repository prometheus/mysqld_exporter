// Copyright 2022 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"database/sql/driver"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeSysUserSummaryByStatementLatency(t *testing.T) {
	// Sanity check
	if (ScrapeSysUserSummaryByStatementLatency{}).Name() != "sys.user_summary_by_statement_latency" {
		t.Fatalf("unexpected Name()")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	columns := []string{
		"user",
		"total",
		"total_latency",
		"max_latency",
		"lock_latency",
		"cpu_latency",
		"rows_sent",
		"rows_examined",
		"rows_affected",
		"full_scans",
	}
	rows := sqlmock.NewRows(columns)

	queryResults := [][]driver.Value{
		// user, total, total_latency(ps), max_latency(ps), lock_latency(ps), cpu_latency(ps), rows_sent, rows_examined, rows_affected, full_scans
		{"app", "10", "120", "300", "40", "50", "1000", "2000", "300", "7"},
		{"background", "2", "0", "0", "0", "0", "0", "0", "0", "0"},
	}
	for _, r := range queryResults {
		rows.AddRow(r...)
	}

	// Pass regex as STRING (raw literal); sqlmock compiles it internally.
	mock.ExpectQuery(`(?s)SELECT\s+.*\s+FROM\s+sys\.x\$user_summary_by_statement_latency\s*`).
		WillReturnRows(rows)

	// Expected metrics (emission order per row)
	expected := []MetricResult{}
	for _, r := range queryResults {
		u := r[0].(string)
		parse := func(s string) float64 {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			return f
		}
		total := parse(r[1].(string))
		totalLat := parse(r[2].(string)) / picoSeconds
		maxLat := parse(r[3].(string)) / picoSeconds
		lockLat := parse(r[4].(string)) / picoSeconds
		cpuLat := parse(r[5].(string)) / picoSeconds
		rowsSent := parse(r[6].(string))
		rowsExam := parse(r[7].(string))
		rowsAff := parse(r[8].(string))
		fullScans := parse(r[9].(string))

		lbl := labelMap{"user": u}
		mt := dto.MetricType_GAUGE

		expected = append(expected,
			MetricResult{labels: lbl, value: total, metricType: mt},
			MetricResult{labels: lbl, value: totalLat, metricType: mt},
			MetricResult{labels: lbl, value: maxLat, metricType: mt},
			MetricResult{labels: lbl, value: lockLat, metricType: mt},
			MetricResult{labels: lbl, value: cpuLat, metricType: mt},
			MetricResult{labels: lbl, value: rowsSent, metricType: mt},
			MetricResult{labels: lbl, value: rowsExam, metricType: mt},
			MetricResult{labels: lbl, value: rowsAff, metricType: mt},
			MetricResult{labels: lbl, value: fullScans, metricType: mt},
		)
	}

	ch := make(chan prometheus.Metric)
	go func() {
		if err := (ScrapeSysUserSummaryByStatementLatency{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("scrape error: %s", err)
		}
		close(ch)
	}()

	convey.Convey("Metrics comparison (user_summary_by_statement_latency)", t, func() {
		for i, exp := range expected {
			m, ok := <-ch
			if !ok {
				t.Fatalf("metrics channel closed early at index %d", i)
			}
			got := readMetric(m)
			convey.So(exp, convey.ShouldResemble, got)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %s", err)
	}
}
