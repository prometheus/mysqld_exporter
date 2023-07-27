// Copyright 2023 The Prometheus Authors
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

package config

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/ini.v1"
)

var (
	mycnfIniOpts = ini.LoadOptions{
		// Do not error on nonexistent file to allow empty string as filename input
		Loose: true,
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
	}
)

type MycnfReloader interface {
	Mycnf() Mycnf
	Reload() error
}

type MycnfReloaderOpts struct {
	Logger                       log.Logger
	MycnfPath                    string
	DefaultMysqldAddress         string
	DefaultMysqldUser            string
	DefaultTlsInsecureSkipVerify bool
}

type mycnfReloader struct {
	sync.RWMutex

	file  *ini.File
	opts  *MycnfReloaderOpts
	mycnf Mycnf

	reloadSeconds prometheus.Gauge
	reloadSuccess prometheus.Gauge
}

func NewMycnfReloader(opts *MycnfReloaderOpts) MycnfReloader {
	return &mycnfReloader{
		mycnf:         make(Mycnf),
		opts:          opts,
		reloadSeconds: configReloadSeconds.WithLabelValues("mycnf"),
		reloadSuccess: configReloadSuccess.WithLabelValues("mycnf"),
	}
}

func (r *mycnfReloader) Mycnf() Mycnf {
	r.RLock()
	defer r.RUnlock()
	return r.mycnf
}

func (r *mycnfReloader) Reload() (err error) {
	var host, port string
	defer func() {
		if err != nil {
			r.reloadSuccess.Set(0)
		} else {
			r.reloadSuccess.Set(1)
			r.reloadSeconds.SetToCurrentTime()
		}
	}()

	if r.file, err = ini.LoadSources(
		mycnfIniOpts,
		[]byte("[client]\npassword = ${MYSQLD_EXPORTER_PASSWORD}\n"),
		r.opts.MycnfPath,
	); err != nil {
		return fmt.Errorf("failed to load %s: %w", r.opts.MycnfPath, err)
	}

	if host, port, err = net.SplitHostPort(r.opts.DefaultMysqldAddress); err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	if clientSection := r.file.Section("client"); clientSection != nil {
		if cfgHost := clientSection.Key("host"); cfgHost.String() == "" {
			cfgHost.SetValue(host)
		}
		if cfgPort := clientSection.Key("port"); cfgPort.String() == "" {
			cfgPort.SetValue(port)
		}
		if cfgUser := clientSection.Key("user"); cfgUser.String() == "" {
			cfgUser.SetValue(r.opts.DefaultMysqldUser)
		}
	}

	r.file.ValueMapper = os.ExpandEnv
	m := make(Mycnf)
	for _, sec := range r.file.Sections() {
		sectionName := sec.Name()

		if sectionName == "DEFAULT" {
			continue
		}

		mycnfSection := &MycnfSection{
			TlsInsecureSkipVerify: r.opts.DefaultTlsInsecureSkipVerify,
		}

		// FIXME: this error check seems orphaned
		if err != nil {
			level.Error(r.opts.Logger).Log("msg", "failed to load config", "section", sectionName, "err", err)
			continue
		}

		err = sec.StrictMapTo(mycnfSection)
		if err != nil {
			level.Error(r.opts.Logger).Log("msg", "failed to parse config", "section", sectionName, "err", err)
			continue
		}
		if err := mycnfSection.validateConfig(); err != nil {
			level.Error(r.opts.Logger).Log("msg", "failed to validate config", "section", sectionName, "err", err)
			continue
		}

		m[sectionName] = *mycnfSection
	}

	if len(m) == 0 {
		return fmt.Errorf("no configuration found")
	}
	r.Lock()
	r.mycnf = m
	r.Unlock()
	return nil
}
