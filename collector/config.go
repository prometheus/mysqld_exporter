package collector

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/mysqld_exporter/config"
	"gopkg.in/yaml.v2"
)

var (
	configFromDefaults *config.Config
	configFromFlags    *config.Config
)

func ConfigFromDefaults() *config.Config {
	return configFromDefaults
}

func ConfigFromFile(path string) (*config.Config, error) {
	configFromFile := &config.Config{}

	var bs []byte
	var err error
	if bs, err = os.ReadFile(path); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", path, err)
	}

	if err = yaml.Unmarshal(bs, configFromFile); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if err = configFromFile.Validate(); err != nil {
		return nil, fmt.Errorf("config is invalid %s: %w", path, err)
	}

	return configFromFile, err
}

func ConfigFromFlags() (*config.Config, error) {
	if configFromFlags == nil {
		return nil, errors.New("cannot access config from flags before command-line parsing")
	}
	return configFromFlags, nil
}

func Configure(config *config.Config) error {
	if config == nil {
		return nil
	}
	for _, collector := range config.Collectors {
		// Look up scraper associated with this collector.
		scraper, ok := lookup(collector.Name)
		if !ok {
			return fmt.Errorf("no scraper found with name: %s", collector.Name)
		}

		// Disable or enable the scraper if requested.
		if collector.Enabled != nil {
			enabled := *collector.Enabled
			setEnabled(scraper.Name(), enabled)
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

func makeConfigFromDefaults() *config.Config {
	defaultConfig := &config.Config{}

	for _, scraper := range AllScrapers() {
		enabled := IsScraperEnabled(scraper.Name())
		collector := &config.Collector{}

		collector.Name = scraper.Name()
		collector.Enabled = &enabled

		defaultConfig.Collectors = append(defaultConfig.Collectors, collector)

		cfg, ok := scraper.(Configurable)
		if !ok {
			continue
		}

		for _, argDef := range cfg.ArgDefinitions() {
			name := scraper.Name() + "." + argDef.Name()

			arg := &config.Arg{}
			arg.Name = name
			arg.Value = argDef.DefaultValue()

			collector.Args = append(collector.Args, arg)
		}
	}

	return defaultConfig
}

// makeConfigFromFlags returns a *config.Config populated by user-provided CLI flags.
// The config is not populated untilt he flags are parsed.
func makeConfigFromFlags(flags map[string]*kingpin.FlagClause, setConfigFn func(*config.Config)) {
	configFromFlags := &config.Config{}

	// Process scrapers.
	for _, scraper := range AllScrapers() {
		// Get scraper enablement flag.
		cf, ok := flags["collect."+scraper.Name()]
		if !ok {
			continue
		}

		// Was it enabled by the user?
		enabledByUser := false
		cf.IsSetByUser(&enabledByUser)

		// If so, add collector to config.
		collector := &config.Collector{}
		cf.Action(func(*kingpin.ParseContext) error {
			if !enabledByUser {
				return nil
			}
			collector.Name = scraper.Name()
			collector.Enabled = cf.Bool()
			configFromFlags.Collectors = append(configFromFlags.Collectors, collector)
			return nil
		})

		// Process scraper args.
		cfg, ok := scraper.(Configurable)
		if !ok {
			continue
		}

		for _, argDef := range cfg.ArgDefinitions() {
			// Get scraper arg flag.
			af, ok := flags["collect."+scraper.Name()+"."+argDef.Name()]
			if !ok {
				continue
			}

			// Was it set by the user?
			setByUser := false
			af.IsSetByUser(&setByUser)

			// If so, add arg to collector.
			arg := &config.Arg{}
			af.Action(func(*kingpin.ParseContext) error {
				if !setByUser {
					return nil
				}
				var value interface{}
				switch argDef.DefaultValue().(type) {
				case bool:
					value = af.Bool()
				case int:
					value = af.Int()
				case string:
					value = af.String()
				}
				arg.Value = value
				collector.Args = append(collector.Args, arg)
				return nil
			})
		}
	}

	kingpin.CommandLine.Action(func(*kingpin.ParseContext) error {
		setConfigFn(configFromFlags)
		return nil
	})
}

func makeFlagsFromScraper(s Scraper, enabled bool) map[string]*kingpin.FlagClause {
	flags := make(map[string]*kingpin.FlagClause)

	// Register collector enabled flag.
	name := "collect." + s.Name()
	help := s.Help()
	if enabled {
		help = fmt.Sprintf("%s (Enabled by default)", help)
	}
	ef := kingpin.Flag(name, help)
	ef.Default(strconv.FormatBool(enabled)).Bool()
	flags[name] = ef

	// Register collector args flags.
	cfg, ok := s.(Configurable)
	if !ok {
		return flags
	}
	for _, argDef := range cfg.ArgDefinitions() {
		name := s.Name() + "." + argDef.Name()

		help := argDef.Help()
		af := kingpin.Flag(name, help)

		switch argDef.DefaultValue().(type) {
		case bool:
			enabled := argDef.DefaultValue().(bool)
			af.Default(strconv.FormatBool(enabled)).Bool()
			if enabled {
				af.Help(fmt.Sprintf("%s (Enabled by default)", help))
			}
		case int:
			i := argDef.DefaultValue().(int)
			af.Default(strconv.FormatInt(int64(i), 10)).Int()
		case string:
			af.Default(argDef.DefaultValue().(string)).String()
		}

		flags[name] = af
	}

	return flags
}

func init() {
	configFromDefaults = makeConfigFromDefaults()
	makeConfigFromFlags(allFlags(), func(config *config.Config) {
		configFromFlags = config
	})
}
