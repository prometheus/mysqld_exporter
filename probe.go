// Copyright 2022 The Prometheus Authors
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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/mysqld_exporter/collector"
)

func handleProbe(scrapers []collector.Scraper, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		params := r.URL.Query()
		target := params.Get("target")
		if target == "" {
			http.Error(w, "target is required", http.StatusBadRequest)
			return
		}
		collectParams := r.URL.Query()["collect[]"]

		authModule := params.Get("auth_module")
		if authModule == "" {
			authModule = "client"
		}

		cfg := c.GetConfig()
		cfgsection, ok := cfg.Sections[authModule]
		if !ok {
			level.Error(logger).Log("msg", fmt.Sprintf("Could not find section [%s] from config file", authModule))
			http.Error(w, fmt.Sprintf("Could not find config section [%s]", authModule), http.StatusBadRequest)
			return
		}
		dsn, err := cfgsection.FormDSN(target)
		if err != nil {
			level.Error(logger).Log("msg", fmt.Sprintf("Failed to form dsn from section [%s]", authModule), "err", err)
			http.Error(w, fmt.Sprintf("Error forming dsn from config section [%s]", authModule), http.StatusBadRequest)
			return
		}

		filteredScrapers := filterScrapers(scrapers, collectParams)

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(ctx, dsn, filteredScrapers, logger))

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}
