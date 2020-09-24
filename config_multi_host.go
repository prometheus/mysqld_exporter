package main

import (
    "errors"
    "fmt"
    "os"
    "gopkg.in/yaml.v2"
)

var (
    errUserIsNotSet = errors.New("Field 'User' must exist and cannot be empty")
    errNameIsNotSet = errors.New("Field 'Name' must exist and cannot be empty")
    errPasswordIsNotSet = errors.New("Field 'Password' must exist and cannot be empty")
    errPortIsNotSet = errors.New("Field 'Port' must exist and cannot be empty")

)

//multiHostExporterconfig is the struct containing MySQL connection info.
type multiHostExporterConfig struct{
    Clients []Client `yaml:"clients"`
}

type Client struct {
    Name        string `yaml:"name"`
    User        string `yaml:"user"`
    Password    string `yaml:"password"`
    Port        int    `yaml:"port"`
    SSLCA       string `yaml:"ssl-ca"`
    SSLCert     string `yaml:"ssl-cert"`
    SSLKey      string `yaml:"ssl-key"`
}

// newMultiHostExporterConfig returns a new decoded multiHostExporterConfig struct
func newMultiHostExporterConfig(configPath string) (*multiHostExporterConfig, error) {
    c := &multiHostExporterConfig{}

    f, err := os.Open(configPath)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    d := yaml.NewDecoder(f)
    if err := d.Decode(&c); err != nil {
        return nil, err
    }

    return c, nil
}


// validate validates the multi host config
func (c multiHostExporterConfig) validate() error {
	for _, client := range c.Clients {
        if client.Name == "" {
		    return errNameIsNotSet
	    }
        if client.User == "" {
		    return errUserIsNotSet
	    }
        if client.Password == "" {
		    return errPasswordIsNotSet
	    }
        if client.Port == 0 {
		    return errPortIsNotSet
	    }
    }
	return nil
}

// formDSN returns a dsn for a given host in multi host exporter mode
func (c multiHostExporterConfig) formDSN(h string) (string, error) {
    var dsn string
    var default_client_index *int
    for i, client := range c.Clients {
        // Store the default_client index which could be of use later
        if client.Name == "default_client" {
            ind := i
            default_client_index = &ind
        } else if client.Name == h {
            dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/", client.User, client.Password, h, client.Port)
            if client.SSLCA != "" {
                if tlsErr := customizeTLS(client.SSLCA, client.SSLCert, client.SSLKey); tlsErr != nil {
                    tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
                    return dsn, tlsErr
                }
                dsn = fmt.Sprintf("%s?tls=custom", dsn)
            }
            return dsn, nil
        }
    }
    // Could not find host specific client config. Try default_client
    if default_client_index != nil {
        dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/", c.Clients[*default_client_index].User, c.Clients[*default_client_index].Password, h, c.Clients[*default_client_index].Port)
        client := c.Clients[*default_client_index]
        if client.SSLCA != "" {
            if tlsErr := customizeTLS(client.SSLCA, client.SSLCert, client.SSLKey); tlsErr != nil {
                tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
                return dsn, tlsErr
            }
            dsn = fmt.Sprintf("%s?tls=custom", dsn)
        }
        return dsn, nil
    }
    return "", errors.New("Can't form dsn. Didn't find default client or host specific client")
}
