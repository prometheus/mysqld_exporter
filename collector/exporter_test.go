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
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

const dsn = "root@/mysql"

func TestExporter(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	exporter := New(
		context.Background(),
		connDSN,
		[]Scraper{
			ScrapeGlobalStatus{},
		},
		promslog.NewNopLogger(),
	)

	convey.Convey("Metrics describing", t, func() {
		ch := make(chan *prometheus.Desc)
		go func() {
			exporter.Describe(ch)
			close(ch)
		}()

		for range ch {
		}
	})

	convey.Convey("Metrics collection", t, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			exporter.Collect(ch)
			close(ch)
		}()

		seen := false
		for m := range ch {
			if m.Desc() == mysqlUp {
				seen = true
				got := readMetric(m)
				convey.So(got.value, convey.ShouldEqual, 1)
			}
		}
		convey.SoMsg("mysql_up metric was not collected", seen, convey.ShouldBeTrue)
	})
}

func TestExporterWithOpts(t *testing.T) {
	convey.Convey("DSN changes with options", t, func() {
		convey.Convey("without any option", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?")
			convey.So(exporter.maxIdleConns, convey.ShouldEqual, 1)
			convey.So(exporter.connMaxLifetime, convey.ShouldEqual, 0)
		})

		convey.Convey("SetSlowLogFilter enabled", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
				SetSlowLogFilter(true),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27")
		})

		convey.Convey("EnableLockWaitTimeout enabled and SetLockWaitTimeout", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
				EnableLockWaitTimeout(true),
				SetLockWaitTimeout(30),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?lock_wait_timeout=30")
		})

		convey.Convey("EnableLockWaitTimeout disabled", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
				EnableLockWaitTimeout(false),
				SetLockWaitTimeout(30),
				SetSlowLogFilter(true),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27")
		})

		convey.Convey("All options enabled", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
				EnableLockWaitTimeout(true),
				SetLockWaitTimeout(30),
				SetSlowLogFilter(true),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?lock_wait_timeout=30&log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27")
		})

		convey.Convey("All options with existing query parameter", func() {
			dsnWithParams := "root@/mysql?parseTime=true"
			exporter := New(
				context.Background(),
				dsnWithParams,
				[]Scraper{},
				promslog.NewNopLogger(),
				EnableLockWaitTimeout(true),
				SetLockWaitTimeout(30),
				SetSlowLogFilter(true),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?parseTime=true&lock_wait_timeout=30&log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27")
		})
	})
}

type mockScraper struct {
	name     string
	validate func(ctx context.Context, instance *instance) error
}

func (s *mockScraper) Name() string     { return s.name }
func (s *mockScraper) Help() string     { return "mock scraper for testing" }
func (s *mockScraper) Version() float64 { return 0 }

func (s *mockScraper) Scrape(ctx context.Context, instance *instance, _ chan<- prometheus.Metric, _ *slog.Logger) error {
	if s.validate != nil {
		return s.validate(ctx, instance)
	}
	return nil
}

func TestScrapeContextTimeout(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	const timeout = 5 * time.Second
	scraper := &mockScraper{
		name: "timeout_test",
		validate: func(ctx context.Context, _ *instance) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("scraper context should have a deadline")
				return nil
			}
			remaining := time.Until(deadline)
			if remaining > timeout || remaining < timeout-time.Second {
				t.Errorf("expected deadline ~%v from now, got %v", timeout, remaining)
			}
			return nil
		},
	}

	exporter := New(
		context.Background(),
		connDSN,
		[]Scraper{scraper},
		promslog.NewNopLogger(),
		SetQueryTimeout(timeout),
	)

	ch := make(chan prometheus.Metric)
	go func() {
		exporter.Collect(ch)
		close(ch)
	}()
	for range ch {
	}
}

func TestNewInstanceConnectionPool(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	for _, tt := range []struct {
		name                       string
		maxOpen, maxIdle, wantIdle int
		lifetime                   time.Duration
		wantLifetimeClosed         bool
	}{
		{name: "one idle", maxOpen: 2, maxIdle: 1, wantIdle: 1},
		{name: "configured idle", maxOpen: 4, maxIdle: 3, wantIdle: 3},
		{name: "disabled idle", maxOpen: 2, maxIdle: 0, wantIdle: 0},
		{name: "idle clamped to open limit", maxOpen: 2, maxIdle: 3, wantIdle: 2},
		{name: "expired lifetime", maxOpen: 2, maxIdle: 1, lifetime: time.Nanosecond, wantLifetimeClosed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			inst, err := newInstance(ctx, connDSN, tt.maxOpen, tt.maxIdle, tt.lifetime, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer inst.Close()

			// Reserve the whole pool before returning connections to it.
			var conns []*sql.Conn
			for range tt.maxOpen {
				conn, err := inst.db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = conn.Close() })
				conns = append(conns, conn)
			}
			if tt.wantLifetimeClosed {
				time.Sleep(time.Millisecond)
			}
			for _, conn := range conns {
				if err := conn.Close(); err != nil {
					t.Fatal(err)
				}
			}

			stats := inst.db.Stats()
			if stats.MaxOpenConnections != tt.maxOpen {
				t.Errorf("max open connections = %d, want %d", stats.MaxOpenConnections, tt.maxOpen)
			}
			if stats.Idle != tt.wantIdle {
				t.Errorf("idle connections = %d, want %d", stats.Idle, tt.wantIdle)
			}
			if got := stats.MaxLifetimeClosed > 0; got != tt.wantLifetimeClosed {
				t.Errorf("connections expired = %t, want %t", got, tt.wantLifetimeClosed)
			}
		})
	}
}

func TestSlowScraperDoesNotBlockFastScraper(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	const queryTimeout = 2 * time.Second
	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slowReady := make(chan struct{})
	slowResult := make(chan error, 1)
	fastResult := make(chan error, 1)

	slowScraper := &mockScraper{
		name: "slow",
		validate: func(ctx context.Context, instance *instance) error {
			conn, err := instance.getDB().Conn(ctx)
			if err != nil {
				close(slowReady)
				slowResult <- err
				return err
			}
			defer conn.Close()

			// Signal only after reserving one of the pool's connections.
			close(slowReady)

			var result int
			err = conn.QueryRowContext(ctx, "SELECT SLEEP(10)").Scan(&result)
			slowResult <- err
			return err
		},
	}

	fastScraper := &mockScraper{
		name: "fast",
		validate: func(ctx context.Context, instance *instance) error {
			select {
			case <-slowReady:
			case <-ctx.Done():
				fastResult <- ctx.Err()
				return ctx.Err()
			}

			var result int
			err := instance.getDB().QueryRowContext(ctx, "SELECT 1").Scan(&result)
			fastResult <- err
			return err
		},
	}

	exporter := New(
		testCtx,
		connDSN,
		[]Scraper{slowScraper, fastScraper},
		promslog.NewNopLogger(),
		SetQueryTimeout(queryTimeout),
		SetMaxOpenConns(2),
	)

	ch := make(chan prometheus.Metric)
	collectDone := make(chan struct{})
	go func() {
		exporter.Collect(ch)
		close(ch)
	}()
	go func() {
		for range ch {
		}
		close(collectDone)
	}()

	select {
	case err := <-fastResult:
		if err != nil {
			t.Fatalf("fast query failed behind slow query: %v", err)
		}
	case err := <-slowResult:
		t.Fatalf("slow query completed before fast query: %v", err)
	case <-testCtx.Done():
		t.Fatal("timed out waiting for fast query result")
	}

	select {
	case err := <-slowResult:
		t.Fatalf("slow query completed before fast result was asserted: %v", err)
	default:
	}

	select {
	case err := <-slowResult:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("slow query error = %v, want context deadline exceeded", err)
		}
	case <-testCtx.Done():
		t.Fatal("timed out waiting for slow query result")
	}

	select {
	case <-collectDone:
	case <-testCtx.Done():
		t.Fatal("timed out waiting for collection to finish")
	}
}
