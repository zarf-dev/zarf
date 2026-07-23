package clio

import (
	"fmt"
	"strings"

	"github.com/pkg/profile"

	"github.com/anchore/fangs"
)

type Profile string

type DevelopmentConfig struct {
	Profile Profile `yaml:"profile" json:"profile" mapstructure:"profile"`
}

func (d *DevelopmentConfig) DescribeFields(set fangs.FieldDescriptionSet) {
	set.Add(&d.Profile, "capture resource profiling data (available: [cpu, mem, ...])")
}

func (d *DevelopmentConfig) PostLoad() error {
	if d.Profile != "" {
		p := parseProfile(d.Profile)
		if p == nil {
			return fmt.Errorf("invalid profile: %q", d.Profile)
		}
	}
	return nil
}

func parseProfile(p Profile) func() func() {
	profiler := profileFunc(p)
	if profiler == nil {
		return nil
	}
	return func() func() {
		return profile.Start(profiler).Stop
	}
}

func profileFunc(p Profile) func(*profile.Profile) {
	return profilers()[strings.ToLower(strings.TrimSpace(string(p)))]
}

func profilers() map[string]func(*profile.Profile) {
	return map[string]func(*profile.Profile){
		"cpu":       profile.CPUProfile,
		"mem":       profile.MemProfile,
		"memory":    profile.MemProfile,
		"allocs":    profile.MemProfileAllocs,
		"heap":      profile.MemProfileHeap,
		"threads":   profile.ThreadcreationProfile,
		"mutex":     profile.MutexProfile,
		"block":     profile.BlockProfile,
		"clock":     profile.ClockProfile,
		"goroutine": profile.GoroutineProfile,
		"trace":     profile.TraceProfile,
	}
}
