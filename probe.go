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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/mysqld_exporter/collector"
)

func handleProbe(scrapers []collector.Scraper, logger *slog.Logger) http.HandlerFunc {
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
			logger.Error(fmt.Sprintf("Could not find section [%s] from config file", authModule))
			http.Error(w, fmt.Sprintf("Could not find config section [%s]", authModule), http.StatusBadRequest)
			return
		}
		dsn, err := cfgsection.FormDSN(target)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to form dsn from section [%s]", authModule), "err", err)
			http.Error(w, fmt.Sprintf("Error forming dsn from config section [%s]", authModule), http.StatusBadRequest)
			return
		}

		// If a timeout is configured via the Prometheus header, add it to the context.
		timeoutSeconds, err := getScrapeTimeoutSeconds(r, *timeoutOffset)
		if err != nil {
			logger.Error("Error getting timeout from Prometheus header", "err", err)
		}
		if timeoutSeconds > 0 {
			// Create new timeout context with request context as parent.
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds*float64(time.Second)))
			defer cancel()
			// Overwrite request with timeout context.
			r = r.WithContext(ctx)
		}

		filteredScrapers := filterScrapers(scrapers, collectParams)

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(ctx, dsn, filteredScrapers, logger,
			collector.EnableLockWaitTimeout(*enableExporterLockTimeout),
			collector.SetLockWaitTimeout(*exporterLockTimeout),
			collector.SetSlowLogFilter(*slowLogFilter),
		))

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}
