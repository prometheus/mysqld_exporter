package collector

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // register driver
	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
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
	enabled, err := RocksDBEnabled(db)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Skip("RocksDB is not enabled, skipping test")
	}

	convey.Convey("Metrics collection", t, convey.FailureContinues, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			err := ScrapeEngineRocksDBStatus(db, ch)
			if err != nil {
				t.Error(err)
			}
			close(ch)
		}()

		// check that we found all metrics we expect
		expected := make(map[string]struct{})
		for k := range engineRocksDBStatusTypes {
			expected[k] = struct{}{}
		}
		for m := range ch {
			got := helpers.ReadMetric(m)
			convey.So(expected, convey.ShouldContainKey, got.Help)
			delete(expected, got.Help)
		}
		// two exceptions
		convey.So(expected, convey.ShouldResemble, map[string]struct{}{
			"rocksdb.l0.hit": struct{}{},
			"rocksdb.l1.hit": struct{}{},
		})
	})
}
