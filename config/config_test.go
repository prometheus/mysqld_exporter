package config

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

var (
	disabled = false
	enabled  = true
)

func TestConfigClone(t *testing.T) {
	convey.Convey("Empty config", t, func() {
		orig := &Config{}
		clone := orig.Clone()

		convey.Convey("Clone is not equal to the original", func() {
			convey.So(clone != orig, convey.ShouldBeTrue)
		})

		convey.Convey("Modify the clone does not affect the original", func() {
			clone.Collectors = append(clone.Collectors, &Collector{Name: "a"})
			convey.So(orig.Collectors, convey.ShouldHaveLength, 0)
		})

		convey.Convey("Modify the original does not affect the clone", func() {
			orig.Collectors = append(orig.Collectors, &Collector{Name: "b"})
			convey.So(clone.Collectors, convey.ShouldHaveLength, 0)
		})
	})

	convey.Convey("Non-empty", t, func() {
		orig := &Config{
			Collectors: []*Collector{
				{
					Name: "0",
				},
				{
					Name:    "1",
					Enabled: &disabled,
				},
				{
					Name:    "2",
					Enabled: &enabled,
				},
				{
					Name: "3",
					Args: []*Arg{},
				},
				{
					Name: "4",
					Args: []*Arg{
						{
							Name: "4.0",
						},
						{
							Name:  "4.1",
							Value: true,
						},
						{
							Name:  "4.1",
							Value: 4,
						},
						{
							Name:  "4.1",
							Value: "4",
						},
					},
				},
			},
		}
		clone := orig.Clone()

		convey.Convey("Clone is not equal to the original", func() {
			convey.So(clone != orig, convey.ShouldBeTrue)
		})

		convey.Convey("Clone is the same as the original", func() {
			convey.So(clone, convey.ShouldEqual, orig)
		})

		convey.Convey("Modify the clone does not affect the original", func() {
			clone.Collectors = append(clone.Collectors, &Collector{Name: "5"})
			convey.So(orig.Collectors, convey.ShouldHaveLength, 5)
		})

		convey.Convey("Modify the original does not affect the clone", func() {
			orig.Collectors = append(orig.Collectors, &Collector{Name: "5"})
			convey.So(clone.Collectors, convey.ShouldHaveLength, 5)
		})
	})
}

func TestConfigMerge(t *testing.T) {
	convey.Convey("Nil source config, non-nil target", t, func() {
		target := &Config{}
		var source *Config

		target.Merge(source)

		convey.Convey("Target is not modified", func() {
			convey.So(target, convey.ShouldNotBeNil)
			convey.So(target.Collectors, convey.ShouldBeNil)
		})

		convey.Convey("Source is not modified", func() {
			convey.So(source, convey.ShouldBeNil)
		})
	})

	convey.Convey("Source config with zero collectors, target config with nil collectors", t, func() {
		target := &Config{}
		source := &Config{
			Collectors: []*Collector{},
		}

		target.Merge(source)

		convey.Convey("Target collectors is zero", func() {
			convey.So(target.Collectors, convey.ShouldNotBeNil)
			convey.So(target.Collectors, convey.ShouldHaveLength, 0)
		})
	})

	convey.Convey("Source config with nil collectors, target config with zero collectors", t, func() {
		target := &Config{
			Collectors: []*Collector{},
		}
		source := &Config{}

		target.Merge(source)

		convey.Convey("Target collectors is zero", func() {
			convey.So(target.Collectors, convey.ShouldNotBeNil)
			convey.So(target.Collectors, convey.ShouldHaveLength, 0)
		})
	})

	convey.Convey("Source and target configs have distinct collectors", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{Name: "a"},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{Name: "b"},
			},
		}

		target.Merge(source)

		convey.Convey("Adds source collector to target", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 2)
			convey.So(target.Collectors[0].Name, convey.ShouldEqual, "a")
			convey.So(target.Collectors[1].Name, convey.ShouldEqual, "b")
		})
	})

	convey.Convey("Source collector does not specify enabled", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name:    "a",
					Enabled: &enabled,
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
				},
			},
		}

		target.Merge(source)

		convey.Convey("Does not change target", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Name, convey.ShouldEqual, "a")
			convey.So(target.Collectors[0].Enabled, convey.ShouldNotBeNil)
			convey.So(target.Collectors[0].Enabled, convey.ShouldPointTo, &enabled)
		})
	})

	convey.Convey("Source collector specifies enabled", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name:    "a",
					Enabled: &enabled,
				},
			},
		}

		target.Merge(source)

		convey.Convey("Does not change target", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Name, convey.ShouldEqual, "a")
			convey.So(target.Collectors[0].Enabled, convey.ShouldNotBeNil)
			convey.So(
				target.Collectors[0].Enabled,
				convey.ShouldNotPointTo,
				source.Collectors[0].Enabled,
			)
			convey.So(*target.Collectors[0].Enabled, convey.ShouldEqual, enabled)
		})
	})

	convey.Convey("Source and target collector specify different enabled values", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name:    "a",
					Enabled: &disabled,
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name:    "a",
					Enabled: &enabled,
				},
			},
		}

		target.Merge(source)

		convey.Convey("Does not change target", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Name, convey.ShouldEqual, "a")
			convey.So(target.Collectors[0].Enabled, convey.ShouldNotBeNil)
			convey.So(*target.Collectors[0].Enabled, convey.ShouldEqual, enabled)
			convey.So(
				target.Collectors[0].Enabled,
				convey.ShouldNotPointTo,
				source.Collectors[0].Enabled,
			)
		})
	})

	convey.Convey("Source collector with zero args, target config with nil args", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Args: []*Arg{},
				},
			},
		}

		target.Merge(source)

		convey.Convey("Target collector args is zero", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args, convey.ShouldNotBeNil)
			convey.So(target.Collectors[0].Args, convey.ShouldHaveLength, 0)
		})
	})
	convey.Convey("Source collector with nil args, target config with zero args", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Args: []*Arg{},
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{},
			},
		}

		target.Merge(source)

		convey.Convey("Target collector args is zero", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args, convey.ShouldNotBeNil)
			convey.So(target.Collectors[0].Args, convey.ShouldHaveLength, 0)
		})
	})

	convey.Convey("Source and target collector specify distinct args", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name: "a.a",
						},
					},
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name: "a.b",
						},
					},
				},
			},
		}

		target.Merge(source)

		convey.Convey("Adds source collector args to target", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args, convey.ShouldHaveLength, 2)
			convey.So(target.Collectors[0].Args[0].Name, convey.ShouldEqual, "a.a")
			convey.So(target.Collectors[0].Args[1].Name, convey.ShouldEqual, "a.b")
		})
	})

	convey.Convey("Source collector args nil value", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name:  "a.a",
							Value: 1,
						},
					},
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name: "a.a",
						},
					},
				},
			},
		}

		target.Merge(source)

		convey.Convey("Target is not changed", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args[0].Name, convey.ShouldEqual, "a.a")
			convey.So(target.Collectors[0].Args[0].Value, convey.ShouldEqual, 1)
		})
	})

	convey.Convey("Source and target collector arg with different value", t, func() {
		target := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name:  "a.a",
							Value: 1,
						},
					},
				},
			},
		}
		source := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name:  "a.a",
							Value: 2,
						},
					},
				},
			},
		}

		target.Merge(source)

		convey.Convey("Target is not changed", func() {
			convey.So(target.Collectors, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args, convey.ShouldHaveLength, 1)
			convey.So(target.Collectors[0].Args[0].Name, convey.ShouldEqual, "a.a")
			convey.So(target.Collectors[0].Args[0].Value, convey.ShouldEqual, 2)
		})
	})
}

func TestConfigValidate(t *testing.T) {
	convey.Convey("Valid config", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
				},
				{
					Name:    "b",
					Enabled: &enabled,
					Args:    []*Arg{},
				},
				{
					Name:    "c",
					Enabled: &disabled,
					Args: []*Arg{
						{
							Name: "1",
						},
					},
				},
				{
					Name: "d",
					Args: []*Arg{
						{
							Name: "1",
						},
						{
							Name:  "2",
							Value: false,
						},
						{
							Name:  "3",
							Value: false,
						},
						{
							Name:  "4",
							Value: true,
						},
						{
							Name:  "5",
							Value: -1,
						},
						{
							Name:  "6",
							Value: "hello",
						},
					},
				},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeNil)
	})

	convey.Convey("Unnamed collector", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeError)
		convey.So(err.Error(), convey.ShouldEqual, "collector  is invalid: name must not be empty")
	})

	convey.Convey("Config with duplicate collectors", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
				},
				{
					Name: "a",
				},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeError)
		convey.So(err.Error(), convey.ShouldEqual, "duplicate collectors named a")
	})

	convey.Convey("Collector with duplicate args", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name: "a",
						},
						{
							Name: "a",
						},
					},
				},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeError)
		convey.So(
			err.Error(),
			convey.ShouldEqual,
			"collector a is invalid: duplicate args named a",
		)
	})

	convey.Convey("Unnamed arg", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{},
					},
				},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeError)
		convey.So(err.Error(), convey.ShouldEqual, "collector a is invalid: arg  is invalid: name must not be empty")
	})

	convey.Convey("Arg with invalid type", t, func() {
		c := &Config{
			Collectors: []*Collector{
				{
					Name: "a",
					Args: []*Arg{
						{
							Name:  "a",
							Value: &enabled,
						},
					},
				},
			},
		}

		err := c.Validate()

		convey.So(err, convey.ShouldBeError)
		convey.So(
			err.Error(),
			convey.ShouldEqual,
			"collector a is invalid: arg a is invalid: invalid type, must be [bool, int, string]",
		)
	})
}
