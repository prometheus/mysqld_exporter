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

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type standardGo struct {
	c prometheus.Collector
}

func NewStandardGo() Scraper {
	return standardGo{
		c: collectors.NewGoCollector(),
	}
}

// Name of the Scraper.
func (standardGo) Name() string {
	return "standard.go"
}

// Help returns additional information about Scraper.
func (standardGo) Help() string {
	return "Collect exporter Go process metrics"
}

// Version of MySQL from which scraper is available.
func (standardGo) Version() float64 {
	return 0
}

// Scrape collects data.
func (s standardGo) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	s.c.Collect(ch)
	return nil
}

type standardProcess struct {
	c prometheus.Collector
}

func NewStandardProcess() Scraper {
	return standardProcess{
		c: collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	}
}

// Name of the Scraper.
func (standardProcess) Name() string {
	return "standard.process"
}

// Help returns additional information about Scraper.
func (standardProcess) Help() string {
	return "Collect exporter process metrics"
}

// Version of MySQL from which scraper is available.
func (standardProcess) Version() float64 {
	return 0
}

// Scrape collects data.
func (s standardProcess) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	s.c.Collect(ch)
	return nil
}
