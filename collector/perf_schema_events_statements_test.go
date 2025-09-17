// Copyright 2025 The Prometheus Authors
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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePerfEventsStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"SCHEMA_NAME", "DIGEST", "DIGEST_TEXT",
		"COUNT_STAR", "SUM_TIMER_WAIT", "SUM_ERRORS", "SUM_WARNINGS",
		"SUM_ROWS_AFFECTED", "SUM_ROWS_SENT", "SUM_ROWS_EXAMINED",
		"SUM_CREATED_TMP_DISK_TABLES", "SUM_CREATED_TMP_TABLES", "SUM_SORT_MERGE_PASSES",
		"SUM_SORT_ROWS", "SUM_NO_INDEX_USED",
	}

	rows := sqlmock.NewRows(columns).
		AddRow(
			"db1", "digest1", "SELECT * FROM test",
			100, 1000, 1, 2,
			50, 100, 150,
			1, 2, 3,
			100, 1)

	query := fmt.Sprintf(perfEventsStatementsQuery, 120, 86400, 250)
	mock.ExpectQuery(sanitizeQuery(query)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfEventsStatements{}).Scrape(context.Background(), &instance{db: db}, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1000 / picoSeconds, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 0, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 0, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 50, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 150, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1000 / picoSeconds, metricType: dto.MetricType_SUMMARY},
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

func TestScrapePerfEventsStatementsMySQL8028(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	inst := &instance{
		db:      db,
		flavor:  FlavorMySQL,
		version: semver.MustParse("8.0.28"),
	}

	columns := []string{
		"SCHEMA_NAME", "DIGEST", "DIGEST_TEXT",
		"COUNT_STAR", "SUM_TIMER_WAIT",
		"SUM_LOCK_TIME", "SUM_CPU_TIME",
		"SUM_ERRORS", "SUM_WARNINGS",
		"SUM_ROWS_AFFECTED", "SUM_ROWS_SENT", "SUM_ROWS_EXAMINED",
		"SUM_CREATED_TMP_DISK_TABLES", "SUM_CREATED_TMP_TABLES", "SUM_SORT_MERGE_PASSES",
		"SUM_SORT_ROWS", "SUM_NO_INDEX_USED",
		"QUANTILE_95", "QUANTILE_99", "QUANTILE_999",
	}

	rows := sqlmock.NewRows(columns).
		AddRow(
			"db1", "digest1", "SELECT * FROM test",
			100, 1000,
			30, 50,
			1, 2,
			50, 100, 150,
			1, 2, 3,
			100, 1,
			100, 150, 200)

	query := fmt.Sprintf(perfEventsStatementsQueryMySQL, 120, 86400, 250)
	mock.ExpectQuery(sanitizeQuery(query)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfEventsStatements{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1000 / picoSeconds, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 30 / picoSeconds, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 50 / picoSeconds, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 50, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 150, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 100, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "db1", "digest": "digest1", "digest_text": "SELECT * FROM test"}, value: 1000 / picoSeconds, metricType: dto.MetricType_SUMMARY},
	}

	convey.Convey("MySQL 8.0.28+ metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
