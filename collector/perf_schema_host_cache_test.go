// Copyright The Prometheus Authors
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
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePerfHostCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"connect_errors", "connect_errors_max", "host_blocked_errors", "hosts"}
	rows := sqlmock.NewRows(columns).AddRow(5, 3, 2, 4)
	mock.ExpectQuery(sanitizeQuery(perfHostCacheQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfHostCache{}).Scrape(context.Background(), &instance{db: db}, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []struct {
		name  string
		value float64
	}{
		{"mysql_perf_schema_host_cache_connect_errors", 5},
		{"mysql_perf_schema_host_cache_connect_errors_max", 3},
		{"mysql_perf_schema_host_cache_host_blocked_errors", 2},
		{"mysql_perf_schema_host_cache_hosts", 4},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			metric := <-ch
			convey.So(metric.Desc().String(), convey.ShouldContainSubstring, `fqName: "`+expect.name+`"`)
			got := readMetric(metric)
			convey.So(got, convey.ShouldResemble, MetricResult{labels: labelMap{}, value: expect.value, metricType: dto.MetricType_GAUGE})
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapePerfHostCacheQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	queryErr := errors.New("query failed")
	mock.ExpectQuery(sanitizeQuery(perfHostCacheQuery)).WillReturnError(queryErr)

	ch := make(chan prometheus.Metric, 4)
	err = (ScrapePerfHostCache{}).Scrape(context.Background(), &instance{db: db}, ch, promslog.NewNopLogger())
	if !errors.Is(err, queryErr) {
		t.Errorf("error = %v, want %v", err, queryErr)
	}
	if len(ch) != 0 {
		t.Errorf("got %d metrics, want 0", len(ch))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
