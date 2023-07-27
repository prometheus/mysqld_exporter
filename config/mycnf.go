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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type Mycnf map[string]MycnfSection

type MycnfSection struct {
	User                  string `ini:"user"`
	Password              string `ini:"password"`
	Host                  string `ini:"host"`
	Port                  int    `ini:"port"`
	Socket                string `ini:"socket"`
	SslCa                 string `ini:"ssl-ca"`
	SslCert               string `ini:"ssl-cert"`
	SslKey                string `ini:"ssl-key"`
	TlsInsecureSkipVerify bool   `ini:"ssl-skip-verfication"`
	Tls                   string `ini:"tls"`
}

func (m MycnfSection) FormDSN(target string) (dsn string, err error) {
	config := mysql.NewConfig()
	config.User = m.User
	config.Passwd = m.Password
	config.Net = "tcp"
	if target == "" {
		if m.Socket == "" {
			host := "127.0.0.1"
			if m.Host != "" {
				host = m.Host
			}
			port := "3306"
			if m.Port != 0 {
				port = strconv.Itoa(m.Port)
			}
			config.Addr = net.JoinHostPort(host, port)
		} else {
			config.Net = "unix"
			config.Addr = m.Socket
		}
	} else if prefix := "unix://"; strings.HasPrefix(target, prefix) {
		config.Net = "unix"
		config.Addr = target[len(prefix):]
	} else {
		if _, _, err = net.SplitHostPort(target); err != nil {
			return "", fmt.Errorf("failed to parse target: %s", err)
		}
		config.Addr = target
	}

	if m.TlsInsecureSkipVerify {
		config.TLSConfig = "skip-verify"
	} else {
		config.TLSConfig = m.Tls
		if m.SslCa != "" {
			if err := m.CustomizeTLS(); err != nil {
				err = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %w", err)
				return "", err
			}
			config.TLSConfig = "custom"
		}
	}

	return config.FormatDSN(), nil
}

func (m MycnfSection) CustomizeTLS() error {
	var tlsCfg tls.Config
	caBundle := x509.NewCertPool()
	pemCA, err := os.ReadFile(m.SslCa)
	if err != nil {
		return err
	}
	if ok := caBundle.AppendCertsFromPEM(pemCA); ok {
		tlsCfg.RootCAs = caBundle
	} else {
		return fmt.Errorf("failed parse pem-encoded CA certificates from %s", m.SslCa)
	}
	if m.SslCert != "" && m.SslKey != "" {
		certPairs := make([]tls.Certificate, 0, 1)
		keypair, err := tls.LoadX509KeyPair(m.SslCert, m.SslKey)
		if err != nil {
			return fmt.Errorf("failed to parse pem-encoded SSL cert %s or SSL key %s: %w",
				m.SslCert, m.SslKey, err)
		}
		certPairs = append(certPairs, keypair)
		tlsCfg.Certificates = certPairs
	}
	tlsCfg.InsecureSkipVerify = m.TlsInsecureSkipVerify
	mysql.RegisterTLSConfig("custom", &tlsCfg)
	return nil
}

func (m MycnfSection) validateConfig() error {
	if m.User == "" {
		return fmt.Errorf("no user specified in section or parent")
	}

	return nil
}
