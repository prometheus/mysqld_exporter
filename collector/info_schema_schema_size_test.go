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
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
)

// schemaSizeQuery contains a "+", which sanitizeQuery does not escape.
var schemaSizeQueryRegexp = "^" + regexp.QuoteMeta(strings.Join(strings.Fields(schemaSizeQuery), " ")) + "$"

func scrapeSchemaSizeMetrics(ctx context.Context, inst *instance) ([]prometheus.Metric, error) {
	ch := make(chan prometheus.Metric)
	result := make(chan error, 1)

	go func() {
		result <- (ScrapeSchemaSize{}).Scrape(ctx, inst, ch, promslog.NewNopLogger())
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	return metrics, <-result
}

func TestScrapeSchemaSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schemas []string
		sizes   []float64
	}{
		{
			name:    "several schemas including empty and large ones",
			schemas: []string{"shop", "empty", "app"},
			sizes:   []float64{2637824, 0, 1 << 40},
		},
		{
			name: "no schemas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })

			rows := sqlmock.NewRows([]string{"TABLE_SCHEMA", "SIZE"})
			for i, schema := range tt.schemas {
				rows.AddRow(schema, tt.sizes[i])
			}
			mock.ExpectQuery(schemaSizeQueryRegexp).WillReturnRows(rows).RowsWillBeClosed()

			metrics, err := scrapeSchemaSizeMetrics(t.Context(), &instance{db: db})
			if err != nil {
				t.Fatal(err)
			}
			if len(metrics) != len(tt.schemas) {
				t.Fatalf("got %d metrics, want %d", len(metrics), len(tt.schemas))
			}
			for i, schema := range tt.schemas {
				if got, want := metrics[i].Desc().String(), infoSchemaSchemaSizeDesc.String(); got != want {
					t.Errorf("descriptor = %s, want %s", got, want)
				}
				want := MetricResult{labels: labelMap{"schema": schema}, value: tt.sizes[i], metricType: dto.MetricType_GAUGE}
				if diff := cmp.Diff(want, readMetric(metrics[i]), cmp.AllowUnexported(MetricResult{})); diff != "" {
					t.Errorf("metric mismatch (-want +got):\n%s", diff)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScrapeSchemaSizeErrors(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("query failed")
	iterationErr := errors.New("iteration failed")
	closeErr := errors.New("close failed")

	tests := []struct {
		name      string
		rows      *sqlmock.Rows
		queryErr  error
		wantErr   error
		wantScan  bool
		wantCount int
	}{
		{
			name:     "query",
			queryErr: queryErr,
			wantErr:  queryErr,
		},
		{
			name:     "null schema",
			rows:     sqlmock.NewRows([]string{"schema", "size"}).AddRow(nil, 1),
			wantScan: true,
		},
		{
			name:     "invalid size",
			rows:     sqlmock.NewRows([]string{"schema", "size"}).AddRow("app", "invalid"),
			wantScan: true,
		},
		{
			name: "iteration after a valid row",
			rows: sqlmock.NewRows([]string{"schema", "size"}).
				AddRow("shop", 2).AddRow("app", 1).RowError(1, iterationErr),
			wantErr:   iterationErr,
			wantCount: 1,
		},
		{
			name:    "close at end of rows",
			rows:    sqlmock.NewRows([]string{"schema", "size"}).CloseError(closeErr),
			wantErr: closeErr,
		},
		{
			name:     "scan and close",
			rows:     sqlmock.NewRows([]string{"schema", "size"}).AddRow("app", "invalid").CloseError(closeErr),
			wantErr:  closeErr,
			wantScan: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })

			query := mock.ExpectQuery(schemaSizeQueryRegexp)
			if tt.queryErr != nil {
				query.WillReturnError(tt.queryErr)
			} else {
				query.WillReturnRows(tt.rows).RowsWillBeClosed()
			}

			metrics, err := scrapeSchemaSizeMetrics(t.Context(), &instance{db: db})
			if err == nil {
				t.Fatal("expected scrape error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantScan && !strings.Contains(err.Error(), "Scan error") {
				t.Errorf("error = %v, want scan error", err)
			}
			if len(metrics) != tt.wantCount {
				t.Errorf("got %d metrics, want %d", len(metrics), tt.wantCount)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScrapeSchemaSizeCanceled(t *testing.T) {
	t.Parallel()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	metrics, err := scrapeSchemaSizeMetrics(ctx, &instance{db: db})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics with a canceled context", len(metrics))
	}
}

func TestScrapeSchemaSizeIntegration(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)

	inst, err := newInstance(ctx, connDSN, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	// An InnoDB table allocates pages even when nearly empty, so the schema size is positive.
	db := inst.getDB()
	for _, query := range []string{
		"CREATE DATABASE IF NOT EXISTS mysqld_exporter_test",
		"CREATE TABLE IF NOT EXISTS mysqld_exporter_test.schema_size (id INT PRIMARY KEY) ENGINE=InnoDB",
		"INSERT IGNORE INTO mysqld_exporter_test.schema_size VALUES (1), (2)",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}

	metrics, err := scrapeSchemaSizeMetrics(ctx, inst)
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]float64)
	for _, metric := range metrics {
		if metric.Desc() != infoSchemaSchemaSizeDesc {
			t.Fatalf("unexpected metric: %s", metric.Desc())
		}
		result := readMetric(metric)
		if result.metricType != dto.MetricType_GAUGE {
			t.Errorf("metric type = %v, want gauge", result.metricType)
		}
		schema := result.labels["schema"]
		if _, ok := got[schema]; ok {
			t.Errorf("duplicate series for schema %q", schema)
		}
		got[schema] = result.value
	}

	// MySQL >= 8.0 caches table sizes (information_schema_stats_expiry), so compare
	// against the same cached statistics rather than against freshly inserted rows.
	var want float64
	if err := db.QueryRowContext(ctx, `
		SELECT SUM(IFNULL(DATA_LENGTH, 0) + IFNULL(INDEX_LENGTH, 0))
		FROM information_schema.tables
		WHERE TABLE_SCHEMA = 'mysqld_exporter_test'
	`).Scan(&want); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"mysql", "performance_schema", "information_schema", "sys"} {
		if _, ok := got[schema]; ok {
			t.Errorf("system schema %q must not be reported", schema)
		}
	}

	size, ok := got["mysqld_exporter_test"]
	if !ok {
		t.Fatalf("no series for schema mysqld_exporter_test in %v", got)
	}
	if size <= 0 || size != want {
		t.Errorf("schema size = %v, want %v (> 0)", size, want)
	}
}
