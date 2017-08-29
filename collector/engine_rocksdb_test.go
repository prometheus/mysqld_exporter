package collector

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // register driver
	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

// getDB waits until MySQL is up and returns opened connection.
func getDB(t testing.TB) *sql.DB {
	var db *sql.DB
	var err error
	for i := 0; i < 20; i++ {
		db, err = sql.Open("mysql", "root@/mysql")
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			return db
		}
		t.Log(err)
		time.Sleep(time.Second)
	}
	t.Fatalf("Failed to get database connection: %s", err)
	panic("not reached")
}

func TestScrapeEngineRocksDBStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	db := getDB(t)

	convey.Convey("Metrics collection", t, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			err := ScrapeEngineRocksDBStatus(db, ch)
			if err != nil {
				t.Error(err)
			}
			close(ch)
		}()
		for m := range ch {
			got := helpers.ReadMetric(m)
			if got.Name == "mysql_engine_rocksdb_rocksdb_bytes_read" {
				convey.So(got.Type, convey.ShouldEqual, dto.MetricType_COUNTER)
				convey.So(got.Value, convey.ShouldBeGreaterThan, 0)
			}
		}
	})
}
