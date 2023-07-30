package config

import (
	"fmt"

	"github.com/prometheus/mysqld_exporter/collector"
)

func Apply(config *Config) error {
	if config == nil {
		return nil
	}
	for _, c := range config.Collectors {
		// Look up s associated with this collector.
		s, ok := collector.LookupScraper(c.Name)
		if !ok {
			return fmt.Errorf("no scraper found with name: %s", c.Name)
		}

		// Disable or enable the scraper if requested.
		if c.Enabled != nil {
			enabled := *c.Enabled
			s.SetEnabled(enabled)
		}

		// Apply arguments if the scraper is configurable.
		if len(c.Args) == 0 {
			continue
		}
		cfg, ok := s.(collector.Configurable)
		if !ok {
			return fmt.Errorf("scraper %s is not configurable", s.Name())
		}
		args := make([]collector.Arg, len(c.Args))
		for i := range c.Args {
			args[i] = collector.NewArg(c.Args[i].Name, c.Args[i].Value)
		}
		if err := cfg.Configure(args...); err != nil {
			return fmt.Errorf("failed to configure scraper %s: %w", s.Name(), err)
		}
	}

	return nil
}
