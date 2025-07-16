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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

// TestScrapeGtidExecuted tests ScrapeGtidExecuted behaviour
func TestScrapeGtidExecuted(t *testing.T) {

	tests := []struct {
		name     string
		gtidSet  string
		expected float64
	}{
		{"empty_set", "", 0},
		{"single_uuid_and_range", `uuid1:1-1000`, 1000},
		{"multiple_uuid_single_range", `uuid1:1-1000,
	uuid1:1001-2000`, 2000},
		{"single_uuid_with_ranges", `uuid1:1-1000,2001-4000`, 3000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("error opening a stub database connection: %s", err)
			}
			defer db.Close()

			inst := &instance{db: db}

			columns := []string{"@@gtid_executed"}
			rows := sqlmock.NewRows(columns).
				AddRow(test.gtidSet)
			mock.ExpectQuery(gtidTransactionCountQuery).
				WithArgs().
				WillReturnRows(rows)

			ch := make(chan prometheus.Metric)
			go func() {
				if err = (ScrapeGtidExecuted{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
					t.Errorf("error calling function on test: %s", err)
				}
				close(ch)
			}()

			counterExpected := []MetricResult{
				{
					labels:     labelMap{},
					value:      test.expected,
					metricType: dto.MetricType_COUNTER,
				},
			}

			convey.Convey("Metrics comparison", t, func() {
				for _, expect := range counterExpected {
					got := readMetric(<-ch)
					convey.So(got, convey.ShouldResemble, expect)
				}
			})

			// Ensure all SQL queries were executed
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})

	}

}
