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
		name      string
		gtidSet   string
		expected  float64
		expectErr bool
	}{
		{"empty_set", "", 0, false},
		{"single_transaction", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1`, 1, false},
		{"single_uuid_and_range", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-1000`, 1000, false},
		{"single_uuid_with_ranges", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-3:11:47-49`, 7, false},
		{"multiple_uuids", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-3,
24BC7856-9C4A-11E1-9D41-80C16E4A511C:1-10`, 13, false},
		{"multiple_uuids_single_line", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-3,24BC7856-9C4A-11E1-9D41-80C16E4A511C:1-10`, 13, false},
		{"tagged_gtid_with_ranges", `3E11FA47-71CA-11E1-9E33-C80AA9429562:Domain_1:1-3:11:47-49`, 7, false},
		{"multiple_tags_for_same_uuid", `3E11FA47-71CA-11E1-9E33-C80AA9429562:Domain_1:1-3:15-21,
3E11FA47-71CA-11E1-9E33-C80AA9429562:Domain_2:8-52`, 55, false},
		{"mixed_tagged_and_untagged_sets", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5,
24BC7856-9C4A-11E1-9D41-80C16E4A511C:Analytics_1:10-14`, 10, false},
		{"missing_uuid", `1-10`, 0, true},
		{"comma_between_intervals", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-3,11-20`, 0, true},
		{"reversed_range", `3E11FA47-71CA-11E1-9E33-C80AA9429562:20-10`, 0, true},
		{"zero_transaction_id", `3E11FA47-71CA-11E1-9E33-C80AA9429562:0`, 0, true},
		{"missing_tagged_interval", `3E11FA47-71CA-11E1-9E33-C80AA9429562:Domain_1`, 0, true},
		{"invalid_tag", `3E11FA47-71CA-11E1-9E33-C80AA9429562:123tag:1-10`, 0, true},
		{"transaction_id_overflow", `3E11FA47-71CA-11E1-9E33-C80AA9429562:1-9223372036854775808`, 0, true},
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

			// buffered so Scrape can complete synchronously: it sends at
			// most one metric before returning.
			ch := make(chan prometheus.Metric, 1)
			scrapeErr := (ScrapeGtidExecuted{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger())
			close(ch)

			if test.expectErr {
				if scrapeErr == nil {
					t.Errorf("expected an error scraping gtid set %q, got none", test.gtidSet)
				}
			} else {
				if scrapeErr != nil {
					t.Fatalf("error calling function on test: %s", scrapeErr)
				}

				counterExpected := MetricResult{
					labels:     labelMap{},
					value:      test.expected,
					metricType: dto.MetricType_COUNTER,
				}

				convey.Convey("Metrics comparison", t, func() {
					got := readMetric(<-ch)
					convey.So(got, convey.ShouldResemble, counterExpected)
				})
			}

			// Ensure all SQL queries were executed
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})

	}

}
