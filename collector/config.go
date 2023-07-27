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

func makeConfigFromDefaults() *config.Config {
	defaultConfig := &config.Config{}

	for scraper, enabled := range All() {
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

			arg := &config.CollectorArg{}
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
	for scraper := range All() {
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
			arg := &config.CollectorArg{}
			af.Action(func(*kingpin.ParseContext) error {
				if !setByUser {
					return nil
				}
				var value interface{}
				switch argDef.Type() {
				case BoolArgType:
					value = af.Bool()
				case IntArgType:
					value = af.Int()
				case StringArgType:
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

func registerFlags() map[string]*kingpin.FlagClause {
	flags := make(map[string]*kingpin.FlagClause)

	for scraper, enabled := range All() {
		// Register collector enabled flag.
		name := "collect." + scraper.Name()
		flags[name] = kingpin.Flag(
			name,
			scraper.Help(),
		).Default(strconv.FormatBool(enabled))

		// Register collector args flags.
		cfg, ok := scraper.(Configurable)
		if !ok {
			continue
		}

		for _, argDef := range cfg.ArgDefinitions() {
			name := scraper.Name() + "." + argDef.Name()

			f := kingpin.Flag(
				name,
				argDef.Help(),
			)

			switch argDef.Type() {
			case BoolArgType:
				f.Default(strconv.FormatBool(argDef.DefaultValue().(bool)))
			case IntArgType:
				i := argDef.DefaultValue().(int)
				f.Default(strconv.FormatInt(int64(i), 10))
			case StringArgType:
				f.Default(argDef.DefaultValue().(string))
			}

			flags[name] = f
		}
	}

	return flags
}

func init() {
	configFromDefaults = makeConfigFromDefaults()
	makeConfigFromFlags(registerFlags(), func(config *config.Config) {
		configFromFlags = config
	})
}
