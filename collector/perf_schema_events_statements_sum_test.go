// Copyright 2026 The Prometheus Authors
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
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePerfEventsStatementsSum(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"SUM_COUNT_STAR",
		"SUM_SUM_CREATED_TMP_DISK_TABLES",
		"SUM_SUM_CREATED_TMP_TABLES",
		"SUM_SUM_ERRORS",
		"SUM_SUM_LOCK_TIME",
		"SUM_SUM_NO_GOOD_INDEX_USED",
		"SUM_SUM_NO_INDEX_USED",
		"SUM_SUM_ROWS_AFFECTED",
		"SUM_SUM_ROWS_EXAMINED",
		"SUM_SUM_ROWS_SENT",
		"SUM_SUM_SELECT_FULL_JOIN",
		"SUM_SUM_SELECT_FULL_RANGE_JOIN",
		"SUM_SUM_SELECT_RANGE",
		"SUM_SUM_SELECT_RANGE_CHECK",
		"SUM_SUM_SELECT_SCAN",
		"SUM_SUM_SORT_MERGE_PASSES",
		"SUM_SUM_SORT_RANGE",
		"SUM_SUM_SORT_ROWS",
		"SUM_SUM_SORT_SCAN",
		"SUM_SUM_TIMER_WAIT",
		"SUM_SUM_WARNINGS",
	}

	// SUM_SUM_TIMER_WAIT larger than math.MaxUint64, as reported in #848.
	// The MySQL driver surfaces such values as []byte decimal strings.
	overflowTimerWait := "36463827771516430384"
	timerWait, err := strconv.ParseFloat(overflowTimerWait, 64)
	if err != nil {
		t.Fatalf("failed to parse overflow timer wait: %s", err)
	}

	rows := sqlmock.NewRows(columns).AddRow(
		100, 1, 2, 3,
		4000, 5, 6, 7,
		8, 9, 10,
		11, 12, 13,
		14, 15, 16, 17,
		18, []byte(overflowTimerWait), 19,
	)
	mock.ExpectQuery(sanitizeQuery(perfEventsStatementsSumQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfEventsStatementsSum{}).Scrape(context.Background(), &instance{db: db}, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 4000, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 5, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 6, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 7, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 8, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 9, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 10, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 11, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 13, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 14, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 15, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 16, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 17, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 18, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: timerWait / picoSeconds, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 19, metricType: dto.MetricType_COUNTER},
	}

	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
