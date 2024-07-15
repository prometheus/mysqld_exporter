// Copyright 2024 PlanetScale, Inc. to appease `make check_license`
package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

type ExtraScraper struct {
	Metric string // suffix after "mysql_extras_" in the fully-qualified metric name
	Query  string // SQL query with named outputs
}

func (es *ExtraScraper) Name() string {
	return fmt.Sprintf("extra_%s", es.Metric)
}

func (es *ExtraScraper) Help() string {
	return fmt.Sprintf("Extra metrics from %s", es.Query)
}

func (*ExtraScraper) Version() float64 {
	return 5.1
}

func (es *ExtraScraper) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	rows, err := db.QueryContext(ctx, es.Query)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	floats, pointers := make([]float64, len(columns)), make([]any, len(columns))
	for i := range columns {
		pointers[i] = &floats[i]
	}

	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return err
		}

		for i := range columns {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "extra", es.Metric),
					es.Help(),
					nil,
					prometheus.Labels{"column": columns[i]},
				),
				prometheus.GaugeValue,
				floats[i],
			)
		}
	}

	return nil
}

var _ Scraper = &ExtraScraper{}

type Extras struct {
	filename string
	interval time.Duration
	logger   log.Logger
	rw       sync.RWMutex // control asynchronous swaps to scrapers
	scrapers []*ExtraScraper
}

func NewExtras(logger log.Logger) (e *Extras, err error) {
	return newExtras(*extrasFilename, *extrasInterval, logger)
}

func newExtras(filename, interval string, logger log.Logger) (e *Extras, err error) {
	e = &Extras{
		filename: filename,
		logger:   logger,
	}
	if interval != "" {
		if e.interval, err = time.ParseDuration(interval); err != nil {
			return
		}
		go e.refresh()
	}
	if err = e.Refresh(); err != nil {
		return
	}
	return
}

func (*Extras) Name() string {
	return "extras"
}

func (*Extras) Help() string {
	return "Extra metrics from arbitrary SQL queries"
}

func (*Extras) Version() float64 {
	return 5.1
}

func (e *Extras) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	for _, s := range e.Scrapers() {
		if err := s.Scrape(ctx, db, ch, logger); err != nil {
			return err
		}
	}
	return nil
}

var _ Scraper = &Extras{}

func (e *Extras) Filename() string {
	return e.filename // no need to lock since it's not exported and we don't mutate it
}

func (e *Extras) Interval() time.Duration {
	return e.interval // no need to lock since it's not exported and we don't mutate it
}

func (e *Extras) Refresh() error {
	if e.filename == "" {
		return nil
	}

	f, err := os.Open(e.filename)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, io.EOF) {
		level.Info(e.logger).Log("msg", "Ignoring error refreshing extras", "err", err)
		return nil // ignore ENOENT and EOF; possibly the next refresh will go better
	} else if err != nil {
		return err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	var scrapers []*ExtraScraper
	if err := dec.Decode(&scrapers); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}

	e.rw.Lock()
	defer e.rw.Unlock()
	e.scrapers = scrapers
	return nil
}

func (e *Extras) Scrapers() []*ExtraScraper {
	e.rw.RLock()
	defer e.rw.RUnlock()
	scrapers := make([]*ExtraScraper, len(e.scrapers))
	copy(scrapers, e.scrapers)
	return scrapers
}

func (e *Extras) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t") // tab makes this easier to test in a Go source file
	enc.Encode(struct {
		Filename, Interval string
		Scrapers           []*ExtraScraper
	}{e.Filename(), e.Interval().String(), e.Scrapers()})
}

func (e *Extras) refresh() {
	for range time.Tick(e.interval) {
		if err := e.Refresh(); err != nil {
			level.Info(e.logger).Log("msg", "Error refreshing extras", "err", err)
		}
	}
}

var (
	extrasFilename = kingpin.Flag(
		"extras.file",
		"path to a file containing extra scraper configurations",
	).String()
	extrasInterval = kingpin.Flag(
		"extras.refresh-interval",
		"interval (in a format acceptable to Go's time.ParseDuration) for refreshing extra scraper configurations",
	).String()
)
