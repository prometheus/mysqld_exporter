package collector

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"gopkg.in/alecthomas/kingpin.v2"
)

const customQueryCounter = `
experiment_garden:
  query: "SELECT fruit, amount FROM experiment.garden;"
  metrics:
    - fruit:
        usage: "LABEL"
        description: "Fruit names"
    - amount:
        usage: "COUNTER"
        description: "Amount fruits in the garden"

`

func TestScrapeCustomQueriesCounter(t *testing.T) {
	convey.Convey("Custom queries counter", t, func() {

		tmpFileName := createTmpFile(t, string(HR), customQueryCounter)
		defer os.Remove(tmpFileName)

		_, err := kingpin.CommandLine.Parse([]string{
			"--collect.custom_query.hr.directory", filepath.Dir(tmpFileName),
		})
		if err != nil {
			t.Fatal(err)
		}

		defer os.Remove(*collectCustomQueryHrDirectory)

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		columns := []string{"fruit", "amount"}
		rows := sqlmock.NewRows(columns).
			AddRow("apple", "10").
			AddRow("cherry", "35").
			AddRow("pear", "42").
			AddRow("plumb", "80")
		mock.ExpectQuery(sanitizeQuery("SELECT fruit, amount FROM experiment.garden;")).WillReturnRows(rows)

		ch := make(chan prometheus.Metric)
		go func() {
			if err = (ScrapeCustomQuery{Resolution: HR}).Scrape(context.Background(), db, ch); err != nil {
				t.Errorf("error calling function on test: %s", err)
			}
			close(ch)
		}()

		counterExpected := []MetricResult{
			{labels: labelMap{"fruit": "apple"}, value: 10, metricType: dto.MetricType_COUNTER},
			{labels: labelMap{"fruit": "cherry"}, value: 35, metricType: dto.MetricType_COUNTER},
			{labels: labelMap{"fruit": "pear"}, value: 42, metricType: dto.MetricType_COUNTER},
			{labels: labelMap{"fruit": "plumb"}, value: 80, metricType: dto.MetricType_COUNTER},
		}
		convey.Convey("Metrics should be resemble", func() {
			for _, expect := range counterExpected {
				got := readMetric(<-ch)
				convey.So(got, convey.ShouldResemble, expect)
			}
		})

		// Ensure all SQL queries were executed
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expections: %s", err)
		}
	})
}

const customQueryDuration = `
experiment_garden:
  query: "SELECT fruit, ripen FROM experiment.garden;"
  metrics:
    - fruit:
        usage: "LABEL"
        description: "Fruit names"
    - amount:
        usage: "DURATION"
        description: "Time to become ripe."

`

func TestScrapeCustomQueriesDuration(t *testing.T) {
	convey.Convey("Custom queries duration", t, func() {

		tmpFileName := createTmpFile(t, string(HR), customQueryDuration)
		defer os.Remove(tmpFileName)

		*collectCustomQueryHrDirectory = filepath.Dir(tmpFileName)
		defer os.Remove(*collectCustomQueryHrDirectory)

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		columns := []string{"fruit", "amount"}
		rows := sqlmock.NewRows(columns).
			AddRow("apple", "2592000000").
			AddRow("cherry", "2692000000").
			AddRow("pear", "2792000000").
			AddRow("plumb", "2892000000")
		mock.ExpectQuery(sanitizeQuery("SELECT fruit, ripen FROM experiment.garden;")).WillReturnRows(rows)

		ch := make(chan prometheus.Metric)
		go func() {
			if err = (ScrapeCustomQuery{Resolution: HR}).Scrape(context.Background(), db, ch); err != nil {
				t.Errorf("error calling function on test: %s", err)
			}
			close(ch)
		}()

		counterExpected := []MetricResult{
			{labels: labelMap{"fruit": "apple"}, value: 2592000000, metricType: dto.MetricType_GAUGE},
			{labels: labelMap{"fruit": "cherry"}, value: 2692000000, metricType: dto.MetricType_GAUGE},
			{labels: labelMap{"fruit": "pear"}, value: 2792000000, metricType: dto.MetricType_GAUGE},
			{labels: labelMap{"fruit": "plumb"}, value: 2892000000, metricType: dto.MetricType_GAUGE},
		}
		convey.Convey("Metrics should be resemble", func() {
			for _, expect := range counterExpected {
				got := readMetric(<-ch)
				convey.So(got, convey.ShouldResemble, expect)
			}
		})

		// Ensure all SQL queries were executed
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expections: %s", err)
		}
	})
}

const customQueryNoDb = `
experiment_garden:
  query: "SELECT fruit, ripen FROM non_existed_experiment.garden;"
  metrics:
    - fruit:
        usage: "LABEL"
        description: "Fruit names"
    - amount:
        usage: "DURATION"
        description: "Time to become ripe."

`

func TestScrapeCustomQueriesDbError(t *testing.T) {
	convey.Convey("Custom queries db error", t, func() {

		tmpFileName := createTmpFile(t, string(HR), customQueryNoDb)
		defer os.Remove(tmpFileName)

		*collectCustomQueryHrDirectory = filepath.Dir(tmpFileName)
		defer os.Remove(*collectCustomQueryHrDirectory)

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		expectedError := fmt.Errorf("ERROR 1049 (42000): Unknown database 'non_existed_experiment'")
		mock.ExpectQuery(sanitizeQuery("SELECT fruit, ripen FROM non_existed_experiment.garden;")).WillReturnError(expectedError)

		ch := make(chan prometheus.Metric)

		expectedErr := "experiment_garden:error running query on database: experiment_garden, ERROR 1049 (42000): Unknown database 'non_existed_experiment'"
		convey.Convey("Should raise error ", func() {
			err = (ScrapeCustomQuery{Resolution: HR}).Scrape(context.Background(), db, ch)
			convey.So(err, convey.ShouldBeError, expectedErr)
		})
		close(ch)
	})
}

const customQueryIncorrectYaml = `
{"foo": "bar"}
`

func TestScrapeCustomQueriesIncorrectYaml(t *testing.T) {
	convey.Convey("Custom queries incorrect yaml", t, func() {

		tmpFileName := createTmpFile(t, string(HR), customQueryIncorrectYaml)
		defer os.Remove(tmpFileName)

		*collectCustomQueryHrDirectory = filepath.Dir(tmpFileName)
		defer os.Remove(*collectCustomQueryHrDirectory)

		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		ch := make(chan prometheus.Metric)

		convey.Convey("Should raise error ", func() {
			err = (ScrapeCustomQuery{Resolution: HR}).Scrape(context.Background(), db, ch)
			convey.So(err, convey.ShouldBeError, "failed to add custom queries:incorrect yaml format for bar")
		})
		close(ch)

	})
}

func TestScrapeCustomQueriesNoFile(t *testing.T) {
	convey.Convey("Passed as a custom queries unexisted file or path", t, func() {

		*collectCustomQueryHrDirectory = "/wrong/path"

		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		ch := make(chan prometheus.Metric)
		err = (ScrapeCustomQuery{Resolution: HR}).Scrape(context.Background(), db, ch)
		close(ch)
		convey.So(err, convey.ShouldBeError, "failed read dir \"/wrong/path\" for custom query. reason: open /wrong/path: no such file or directory")
	})
}

func createTmpFile(t *testing.T, resolution, content string) string {
	// Create our Temp File
	tempDir := os.TempDir() + "/" + resolution
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Cannot create temporary directory: %s", err)
	}
	tmpFile, err := ioutil.TempFile(tempDir, "custom_queries.*.yaml")
	if err != nil {
		t.Fatalf("Cannot create temporary file: %s", err)
	}

	// Example writing to the file
	_, err = tmpFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %s", err)
	}
	return tmpFile.Name()
}
