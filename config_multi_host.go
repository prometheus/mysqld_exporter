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
	"errors"
	"fmt"
	"gopkg.in/ini.v1"
	"strings"
)

var (
	errclientParentIsNotSet = errors.New("Parent Client must exist and cannot be empty")
	errUserIsNotSet         = errors.New("Field 'User' must exist and cannot be empty")
	errPasswordIsNotSet     = errors.New("Field 'Password' must exist and cannot be empty")
)

func newMultiHostExporterConfig(configPath string) (*ini.File, error) {
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

func validateMultiHostExporterConfig(cfg *ini.File) error {
	var clientParent *ini.Section
	var err error

	if clientParent, err = cfg.GetSection("client"); err != nil {
		return errclientParentIsNotSet
	}
	if !clientParent.HasKey("password") || clientParent.Key("password").String() == "" {
		return errPasswordIsNotSet
	}
	if !clientParent.HasKey("user") || clientParent.Key("user").String() == "" {
		return errUserIsNotSet
	}

	for _, clientChild := range clientParent.ChildSections() {
		if !clientChild.HasKey("password") || clientParent.Key("password").String() == "" {
			return errPasswordIsNotSet
		}
		if !clientChild.HasKey("user") || clientParent.Key("user").String() == "" {
			return errUserIsNotSet
		}
	}

	return nil
}

func formMultiHostExporterDSN(hostPort string, cfg *ini.File) (string, error) {
	var dsn, host, port string
	var client *ini.Section
	var err error

	targetPort := strings.Split(hostPort, ":")
	host = targetPort[0]
	if len(targetPort) > 1 {
		port = targetPort[1]
	} else {
		port = "3306"
	}

	if client, err = cfg.GetSection(fmt.Sprintf("client.%s", host)); err != nil {
		// Didn't find host specific client, try default client
		if client, err = cfg.GetSection("client"); err != nil {
			return "", errors.New("Can't form dsn. Didn't find default client or host specific client")
		}
	}

	user := client.Key("user").String()
	password := client.Key("password").String()
	dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)

	if client.HasKey("ssl-ca") && client.Key("ssl-ca").String() != "" {
		if tlsErr := customizeTLS(client.Key("ssl-ca").String(), client.Key("ssl-cert").String(), client.Key("ssl-key").String()); tlsErr != nil {
			tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
			return dsn, tlsErr
		}
		dsn = fmt.Sprintf("%s?tls=custom", dsn)
	}

	return dsn, nil
}
