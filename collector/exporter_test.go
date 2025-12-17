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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

const dsn = "root@/mysql"

func TestExporter(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	exporter := New(
		context.Background(),
		dsn,
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

		for m := range ch {
			got := readMetric(m)
			if got.labels[model.MetricNameLabel] == "mysql_up" {
				convey.So(got.value, convey.ShouldEqual, 1)
			}
		}
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

		convey.Convey("SetInformationSchemaStatsExpiry", func() {
			exporter := New(
				context.Background(),
				dsn,
				[]Scraper{},
				promslog.NewNopLogger(),
				SetInformationSchemaStatsExpiry(120),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?information_schema_stats_expiry=120")
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
				SetInformationSchemaStatsExpiry(0),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?lock_wait_timeout=30&log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27&information_schema_stats_expiry=0")
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
				SetInformationSchemaStatsExpiry(0),
			)
			convey.So(exporter.dsn, convey.ShouldEqual, "root@/mysql?parseTime=true&lock_wait_timeout=30&log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27&information_schema_stats_expiry=0")
		})
	})
}
