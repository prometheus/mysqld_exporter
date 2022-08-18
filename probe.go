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

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/mysqld_exporter/collector"
	"gopkg.in/ini.v1"
)

func handleProbe(metrics collector.Metrics, scrapers []collector.Scraper, cfg *ini.File, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var section *ini.Section
		var dsn string
		var err error
		var authModule string

		ctx := r.Context()
		params := r.URL.Query()
		target := params.Get("target")
		if target == "" {
			http.Error(w, "target is required", http.StatusBadRequest)
			return
		}
		collectParams := r.URL.Query()["collect[]"]

		if authModule = params.Get("auth_module"); authModule == "" {
			authModule = "client"
		}

		if section, err = validateMyConfig(cfg, authModule); err != nil {
			level.Error(logger).Log("msg", "Error parsing my.cnf", "file", *configMycnf, "err", err)
			http.Error(w, fmt.Sprintf("Error parsing config section [%s]", authModule), http.StatusBadRequest)
			return
		}
		if dsn, err = formDSN(target, section); err != nil {
			level.Error(logger).Log("msg", "Error forming dsn", "file", *configMycnf, "target", target, "err", err)
			http.Error(w, fmt.Sprintf("Error forming dsn for %s", target), http.StatusBadRequest)
			return
		}

		probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "probe_success",
			Help: "Displays whether or not the probe was a success",
		})
		probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "probe_duration_seconds",
			Help: "Returns how long the probe took to complete in seconds",
		})

		filteredScrapers := filterScrapers(scrapers, collectParams)

		start := time.Now()
		registry := prometheus.NewRegistry()
		registry.MustRegister(probeSuccessGauge)
		registry.MustRegister(probeDurationGauge)
		registry.MustRegister(collector.New(ctx, dsn, metrics, filteredScrapers, logger))

		if err != nil {
			probeSuccessGauge.Set(0)
			probeDurationGauge.Set(time.Since(start).Seconds())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		duration := time.Since(start).Seconds()
		probeDurationGauge.Set(duration)

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}
