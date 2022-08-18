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
	"strings"

	"gopkg.in/ini.v1"
)

func newMyConfig(configPath string) (*ini.File, error) {
	opts := ini.LoadOptions{
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
	}

	var cfg *ini.File
	var err error
	if cfg, err = ini.LoadSources(opts, configPath); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateMyConfig(cfg *ini.File, section string) (*ini.Section, error) {
	var client *ini.Section
	var err error

	if client, err = cfg.GetSection(section); err != nil {
		return nil, fmt.Errorf("configuration section not found - [%s]", section)
	}
	if !client.HasKey("user") || client.Key("user").String() == "" {
		return nil, fmt.Errorf("no user specified under [%s]", section)
	}
	if !client.HasKey("password") || client.Key("password").String() == "" {
		return nil, fmt.Errorf("no password specified under [%s]", section)
	}

	return client, nil
}

func formDSN(target string, cfg *ini.Section) (string, error) {
	var dsn, host, port string

	user := cfg.Key("user").String()
	password := cfg.Key("password").String()
	if target == "" {
		host := cfg.Key("host").MustString("localhost")
		port := cfg.Key("port").MustUint(3306)
		socket := cfg.Key("socket").String()
		if socket != "" {
			dsn = fmt.Sprintf("%s:%s@unix(%s)/", user, password, socket)
		} else {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
		}
	} else {
		targetPort := strings.Split(target, ":")
		host = targetPort[0]
		if len(targetPort) > 1 {
			port = targetPort[1]
		} else {
			port = "3306"
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)
	}

	if cfg.HasKey("ssl-ca") && cfg.Key("ssl-ca").String() != "" {
		if tlsErr := customizeTLS(cfg.Key("ssl-ca").String(), cfg.Key("ssl-cert").String(), cfg.Key("ssl-key").String()); tlsErr != nil {
			tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
			return dsn, tlsErr
		}
		dsn = fmt.Sprintf("%s?tls=custom", dsn)
	}

	return dsn, nil
}
