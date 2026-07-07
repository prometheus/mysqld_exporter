// Copyright 2021 The Prometheus Authors
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
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"regexp"
	"testing"
)

func TestScrapeTransaction(t *testing.T) {
	_, err := kingpin.CommandLine.Parse([]string{
		"--collect.info_schema.processlist.min_time=0",
	})
	if err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}
	query := fmt.Sprintf(infoSchemaInnodbTRX, 0)
	columns := []string{"count"}
	rows := sqlmock.NewRows(columns).AddRow(0)
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)
	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeTransaction{}).Scrape(context.Background(), inst, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling scrapeTransaction: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
