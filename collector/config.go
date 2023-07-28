package collector

import (
	"fmt"

	"github.com/prometheus/mysqld_exporter/config"
)

func Configure(config *config.Config) error {
	if config == nil {
		return nil
	}
	for _, collector := range config.Collectors {
		// Look up scraper associated with this collector.
		scraper, ok := LookupScraper(collector.Name)
		if !ok {
			return fmt.Errorf("no scraper found with name: %s", collector.Name)
		}

		// Disable or enable the scraper if requested.
		if collector.Enabled != nil {
			enabled := *collector.Enabled
			setScraperEnabled(scraper.Name(), enabled)
		}

		// Apply arguments if the scraper is configurable.
		if len(collector.Args) == 0 {
			continue
		}
		cfg, ok := scraper.(Configurable)
		if !ok {
			return fmt.Errorf("scraper %s is not configurable", scraper.Name())
		}
		args := make([]Arg, len(collector.Args))
		for i := range collector.Args {
			arg := &arg{}
			arg.name = collector.Args[i].Name
			arg.value = collector.Args[i].Value
		}
		if err := cfg.Configure(args...); err != nil {
			return fmt.Errorf("failed to configure scraper %s: %w", scraper.Name(), err)
		}
	}

	return nil
}
