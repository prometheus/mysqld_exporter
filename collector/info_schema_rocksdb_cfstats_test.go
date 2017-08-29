package collector

import (
	"testing"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeRocksDBCFStats(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	db := getDB(t)
	if !rocksDBEnabled(db, t) {
		t.Skip("RocksDB is not enabled, skipping test")
	}

	convey.Convey("Metrics collection", t, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			err := ScrapeRocksDBCFStats(db, ch)
			if err != nil {
				t.Error(err)
			}
			close(ch)
		}()
		for m := range ch {
			got := helpers.ReadMetric(m)
			if got.Name == "mysql_rocksdb_cfstats_cur_size_all_mem_tables" {
				convey.So(got.Type, convey.ShouldEqual, dto.MetricType_UNTYPED)
				convey.So(got.Value, convey.ShouldBeGreaterThan, 0)
				convey.So(got.Labels, convey.ShouldContainKey, "name")
			}
		}
	})
}
