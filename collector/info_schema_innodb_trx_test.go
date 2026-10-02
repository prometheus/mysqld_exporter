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
	"database/sql"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
)

func scrapeInnodbTrxMetrics(ctx context.Context, inst *instance) ([]prometheus.Metric, error) {
	ch := make(chan prometheus.Metric)
	result := make(chan error, 1)

	go func() {
		result <- (ScrapeInnodbTrx{}).Scrape(ctx, inst, ch, promslog.NewNopLogger())
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	return metrics, <-result
}

func assertInnodbTrxMetric(t *testing.T, metric prometheus.Metric, age bool, state string, value float64) {
	t.Helper()

	desc := innodbTrxTransactionsDesc.String()
	if age {
		desc = innodbTrxOldestTransactionDesc.String()
	}
	if got := metric.Desc().String(); got != desc {
		t.Errorf("descriptor = %s, want %s", got, desc)
	}

	want := MetricResult{labels: labelMap{"state": state}, value: value, metricType: dto.MetricType_GAUGE}
	if diff := cmp.Diff(want, readMetric(metric), cmp.AllowUnexported(MetricResult{})); diff != "" {
		t.Errorf("metric mismatch (-want +got):\n%s", diff)
	}
}

func TestScrapeInnodbTrx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		states []string
		counts []float64
		ages   []float64
	}{
		{
			name:   "all states including young transactions and large counts",
			states: []string{"RUNNING", "LOCK WAIT", "ROLLING BACK", "COMMITTING"},
			counts: []float64{1 << 32, 3, 2, 1},
			ages:   []float64{90000, 120, 7, 0},
		},
		{
			name: "no transactions",
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

			rows := sqlmock.NewRows([]string{"trx_state", "transactions", "oldest_transaction_seconds"})
			for i, state := range tt.states {
				rows.AddRow(state, tt.counts[i], tt.ages[i])
			}
			mock.ExpectQuery("^" + sanitizeQuery(innodbTrxQuery) + "$").WillReturnRows(rows).RowsWillBeClosed()

			metrics, err := scrapeInnodbTrxMetrics(t.Context(), &instance{db: db})
			if err != nil {
				t.Fatal(err)
			}
			if len(metrics) != 2*len(tt.states) {
				t.Fatalf("got %d metrics, want %d", len(metrics), 2*len(tt.states))
			}
			for i, state := range tt.states {
				assertInnodbTrxMetric(t, metrics[2*i], false, state, tt.counts[i])
				assertInnodbTrxMetric(t, metrics[2*i+1], true, state, tt.ages[i])
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScrapeInnodbTrxErrors(t *testing.T) {
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
			name:     "null state",
			rows:     sqlmock.NewRows([]string{"state", "count", "age"}).AddRow(nil, 1, 2),
			wantScan: true,
		},
		{
			name:     "invalid count",
			rows:     sqlmock.NewRows([]string{"state", "count", "age"}).AddRow("RUNNING", "invalid", 2),
			wantScan: true,
		},
		{
			name:     "null age",
			rows:     sqlmock.NewRows([]string{"state", "count", "age"}).AddRow("RUNNING", 1, nil),
			wantScan: true,
		},
		{
			name: "iteration after a valid row",
			rows: sqlmock.NewRows([]string{"state", "count", "age"}).
				AddRow("RUNNING", 2, 7).AddRow("LOCK WAIT", 1, 3).RowError(1, iterationErr),
			wantErr:   iterationErr,
			wantCount: 2,
		},
		{
			name:    "close at end of rows",
			rows:    sqlmock.NewRows([]string{"state", "count", "age"}).CloseError(closeErr),
			wantErr: closeErr,
		},
		{
			name:     "scan and close",
			rows:     sqlmock.NewRows([]string{"state", "count", "age"}).AddRow("RUNNING", "invalid", 2).CloseError(closeErr),
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

			query := mock.ExpectQuery("^" + sanitizeQuery(innodbTrxQuery) + "$")
			if tt.queryErr != nil {
				query.WillReturnError(tt.queryErr)
			} else {
				query.WillReturnRows(tt.rows).RowsWillBeClosed()
			}

			metrics, err := scrapeInnodbTrxMetrics(t.Context(), &instance{db: db})
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

func TestScrapeInnodbTrxCanceled(t *testing.T) {
	t.Parallel()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	metrics, err := scrapeInnodbTrxMetrics(ctx, &instance{db: db})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if len(metrics) != 0 {
		t.Errorf("got %d metrics with a canceled context", len(metrics))
	}
}

func TestScrapeInnodbTrxIntegration(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)

	// Allow three open transactions plus a separate connection for scraping.
	inst, err := newInstance(ctx, connDSN, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	// Two rows let transactions run independently before we introduce lock contention.
	db := inst.getDB()
	for _, query := range []string{
		"CREATE DATABASE IF NOT EXISTS mysqld_exporter_test",
		"CREATE TABLE IF NOT EXISTS mysqld_exporter_test.innodb_trx (id INT PRIMARY KEY) ENGINE=InnoDB",
		"INSERT IGNORE INTO mysqld_exporter_test.innodb_trx VALUES (1), (2)",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}

	var transactions []*sql.Tx
	t.Cleanup(func() {
		// Release blockers first, including on failure, so the waiting query can finish.
		for _, tx := range transactions {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		}
	})

	begin := func() *sql.Tx {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		transactions = append(transactions, tx)
		return tx
	}

	// A locking read keeps the transaction active and holds the row until rollback.
	lockRow := func(tx *sql.Tx, id int) error {
		var got int
		return tx.QueryRowContext(ctx, "SELECT id FROM mysqld_exporter_test.innodb_trx WHERE id = ? FOR UPDATE", id).Scan(&got)
	}

	type transactionMetrics struct {
		count, age float64
	}

	// The database controls transaction age and state visibility, so poll the metrics
	// until the expected counts and ages are visible, bounded by the test timeout.
	waitFor := func(wantCounts map[string]float64, minimumAge float64) map[string]transactionMetrics {
		t.Helper()

		ticker := time.NewTicker(100 * time.Millisecond)
		t.Cleanup(ticker.Stop)

		var got map[string]transactionMetrics
		for {
			metrics, err := scrapeInnodbTrxMetrics(ctx, inst)
			if err != nil {
				t.Fatal(err)
			}

			got = make(map[string]transactionMetrics)
			for _, metric := range metrics {
				result := readMetric(metric)
				state := result.labels["state"]
				values := got[state]
				switch metric.Desc() {
				case innodbTrxTransactionsDesc:
					values.count = result.value
				case innodbTrxOldestTransactionDesc:
					values.age = result.value
				default:
					t.Fatalf("unexpected metric: %s", metric.Desc())
				}
				got[state] = values
			}

			matches := len(got) == len(wantCounts)
			for state, count := range wantCounts {
				matches = matches && got[state].count == count && got[state].age >= minimumAge
			}
			if matches {
				return got
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatalf("transaction metrics = %v, want counts %v and age >= %v", got, wantCounts, minimumAge)
			}
		}
	}

	// Verify the idle baseline, then leave the first transaction holding row 1.
	waitFor(nil, 0)
	older := begin()
	started := time.Now()
	if err := lockRow(older, 1); err != nil {
		t.Fatal(err)
	}

	// Let the first transaction age before opening a younger one on a different row.
	// Both should be counted as RUNNING, with age taken from the older transaction.
	waitFor(map[string]float64{"RUNNING": 1}, 3)
	younger := begin()
	if err := lockRow(younger, 2); err != nil {
		t.Fatal(err)
	}
	got := waitFor(map[string]float64{"RUNNING": 2}, 0)
	// Allow for whole-second database timestamps and time spent scraping.
	elapsed := time.Since(started).Seconds()
	if age := got["RUNNING"].age; age < math.Floor(elapsed)-1 || age > math.Ceil(elapsed)+1 {
		t.Errorf("oldest transaction age = %v, want age near %v seconds", age, elapsed)
	}

	// A third transaction requests the locked row: it enters LOCK WAIT while the two existing transactions remain RUNNING.
	waiting := begin()
	lockResult := make(chan error, 1)
	go func() { lockResult <- lockRow(waiting, 1) }()
	waitFor(map[string]float64{"RUNNING": 2, "LOCK WAIT": 1}, 0)

	// Release the blocking lock, wait for the pending read to finish,
	// then roll back the remaining transactions to return the server to its idle state.
	if err := older.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := <-lockResult; err != nil {
		t.Fatal(err)
	}
	if err := waiting.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := younger.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Completed transactions must disappear from both metric families.
	waitFor(nil, 0)
}
