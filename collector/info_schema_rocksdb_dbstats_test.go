package collector

import (
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeRocksDBDBStats(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	db := getDB(t)

	convey.Convey("Metrics collection", t, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			err := ScrapeRocksDBDBStats(db, ch)
			if err != nil {
				t.Error(err)
			}
			close(ch)
		}()
		for m := range ch {
			got := helpers.ReadMetric(m)
			if got.Name == "mysql_rocksdb_dbstats_db_block_cache_usage" {
				convey.So(got.Type, convey.ShouldEqual, dto.MetricType_UNTYPED)
				convey.So(got.Value, convey.ShouldBeGreaterThan, 0)
			}
		}
	})
}
