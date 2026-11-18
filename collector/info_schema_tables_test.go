// Copyright 2018 The Prometheus Authors
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

func TestScrapeTableSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	origDatabases := *tableSchemaDatabases
	*tableSchemaDatabases = "*"
	defer func() { *tableSchemaDatabases = origDatabases }()

	dbListColumns := []string{"SCHEMA_NAME"}
	dbListRows := sqlmock.NewRows(dbListColumns).
		AddRow("test")
	mock.ExpectQuery(sanitizeQuery(dbListQuery)).WillReturnRows(dbListRows)

	columns := []string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "ENGINE", "VERSION", "ROW_FORMAT", "TABLE_ROWS", "DATA_LENGTH", "INDEX_LENGTH", "DATA_FREE", "CREATE_OPTIONS", "TABLE_COLLATION"}
	rows := sqlmock.NewRows(columns).
		AddRow("test", "foo", "BASE TABLE", "InnoDB", 10, "Dynamic", 5, 16384, 0, 0, "", "utf8mb4_general_ci")
	mock.ExpectQuery(sanitizeQuery(tableSchemaQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeTableSchema{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"schema": "test", "table": "foo", "type": "BASE TABLE", "engine": "InnoDB", "row_format": "Dynamic", "create_options": "", "collation": "utf8mb4_general_ci"}, value: 10, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"schema": "test", "table": "foo"}, value: 5, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"schema": "test", "table": "foo", "component": "data_length"}, value: 16384, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"schema": "test", "table": "foo", "component": "index_length"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"schema": "test", "table": "foo", "component": "data_free"}, value: 0, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
