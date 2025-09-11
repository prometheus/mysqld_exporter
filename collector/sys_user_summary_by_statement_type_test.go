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

func TestScrapeSysUserSummaryByStatementType(t *testing.T) {
	if (ScrapeSysUserSummaryByStatementType{}).Name() != "sys.user_summary_by_statement_type" {
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
		"statement",
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
		// user, statement, total, total_latency(ps), max_latency(ps), lock_latency(ps), cpu_latency(ps),
		// rows_sent, rows_examined, rows_affected, full_scans
		{"app", "SELECT", "5", "100", "200", "10", "20", "500", "1000", "200", "3"},
		{"app", "INSERT", "2", "50", "80", "5", "8", "50", "0", "2", "0"},
	}
	for _, r := range queryResults {
		rows.AddRow(r...)
	}

	mock.ExpectQuery(`(?s)SELECT\s+.*\s+FROM\s+sys\.x\$user_summary_by_statement_type\s*`).
		WillReturnRows(rows)

	expected := []MetricResult{}
	for _, r := range queryResults {
		user := r[0].(string)
		stmt := r[1].(string)
		parse := func(s string) float64 {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			return f
		}

		total := parse(r[2].(string))
		totalLat := parse(r[3].(string)) / picoSeconds
		maxLat := parse(r[4].(string)) / picoSeconds
		lockLat := parse(r[5].(string)) / picoSeconds
		cpuLat := parse(r[6].(string)) / picoSeconds
		rowsSent := parse(r[7].(string))
		rowsExam := parse(r[8].(string))
		rowsAff := parse(r[9].(string))
		fullScans := parse(r[10].(string))

		lbl := labelMap{"user": user, "statement": stmt}
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
		if err := (ScrapeSysUserSummaryByStatementType{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("scrape error: %s", err)
		}
		close(ch)
	}()

	convey.Convey("Metrics comparison (user_summary_by_statement_type)", t, func() {
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
