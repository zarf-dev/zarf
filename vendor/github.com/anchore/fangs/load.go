package fangs

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"dario.cat/mergo"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/anchore/go-homedir"
)

func Load(cfg Config, cmd *cobra.Command, configurations ...any) error {
	return loadConfig(cfg, commandFlagRefs(cmd), configurations...)
}

func LoadAt(cfg Config, cmd *cobra.Command, path string, configuration any) error {
	return Load(cfg, cmd, rootAt(cfg, configuration, path))
}

// loadConfig loads all configurations based on the provided settings, configuration variables are loaded
// based on priority of:  viper.Set, flag, env, config, kv, defaults
func loadConfig(cfg Config, flags flagRefs, configurations ...any) error {
	// ensure the config is set up sufficiently
	if cfg.Logger == nil || cfg.Finders == nil {
		return fmt.Errorf("config.Load requires logger and finders to be set, but only has %+v", cfg)
	}

	for _, configuration := range configurations {
		if !isPtr(reflect.TypeOf(configuration)) {
			return fmt.Errorf("config.Load configuration parameters must be a pointers, got: %s -- %v", reflect.TypeOf(configuration).Name(), configuration)
		}
	}

	files, err := findConfigurationFiles(cfg)
	if err != nil {
		return err
	}

	v, err := readConfigurationFiles(cfg, files)
	if err != nil {
		return err
	}

	err = mergeProfiles(cfg, v)
	if err != nil {
		return err
	}

	// loading configurations now will have a merged set of configuration files with the following behavior:
	// each configuration file is loaded, in priority order where the first takes precedence if the same key
	// is defined in multiple files. lists and map configurations will have values appended, and profiles
	// will overwrite values
	for _, configuration := range configurations {
		configureViper(cfg, v, nil, set[reflect.Value]{}, reflect.ValueOf(configuration), flags, []string{})

		// unmarshal fully populated viper object onto config
		err := unmarshalRecover(v, configuration, func(dc *mapstructure.DecoderConfig) {
			dc.TagName = cfg.TagName
			// ZeroFields will use what is present in the config file instead of modifying existing defaults
			dc.ZeroFields = true
		})
		if err != nil {
			return err
		}

		// Convert all populated config options to their internal application values ex: scope string => scopeOpt source.Scope
		err = postLoad(reflect.ValueOf(configuration))
		if err != nil {
			return err
		}
	}

	return nil
}

// unmarshalRecover calls viper.Unmarshal and converts panics from mapstructure into errors.
// mapstructure v2.5.0 auto-initializes squashed pointer structs without checking CanSet, which panics
// for embedded unexported pointer fields like `*private` — a pattern Go reflection cannot support.
func unmarshalRecover(v *viper.Viper, target any, opts ...viper.DecoderConfigOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%v", r)
			if strings.Contains(msg, "unexported field") {
				err = fmt.Errorf("unsupported type for squash: embedded unexported pointer field cannot be set via reflection: %v", r)
				return
			}
			panic(r)
		}
	}()
	return v.Unmarshal(target, opts...)
}

// findConfigurationFiles returns the set of configuration files to use, either directly configured
// or found in search paths, returning files in precedence order
func findConfigurationFiles(cfg Config) (files []string, err error) {
	// load all explicitly configured files specified in cfg.Files and verify they exist
	for _, f := range Flatten(cfg.Files...) {
		f, err = homedir.Expand(f)
		if err != nil {
			return nil, fmt.Errorf("unable to expand path: %s", f)
		}
		if !fileExists(f) {
			return nil, fmt.Errorf("file does not exist: %v", f)
		}
		files = append(files, f)
		if len(files) > 1 && !cfg.MultiFile {
			return nil, fmt.Errorf("multiple configuration files not allowed; got: %v", Flatten(cfg.Files...))
		}
	}

	// only include files in search paths if direct configuration not specified
	if len(files) > 0 {
		return files, nil
	}

	for _, finder := range cfg.Finders {
		for _, file := range finder(cfg) {
			if !fileExists(file) {
				continue
			}
			files = append(files, file)
			if !cfg.MultiFile {
				// if not allowing implicit config inheritance, just return the first file
				return files, nil
			}
		}
	}

	return files, nil
}

// readConfigurationFiles reads all configurations, appending slice values
func readConfigurationFiles(cfg Config, files []string) (v *viper.Viper, err error) {
	v = newViper(cfg)

	for _, f := range files {
		newV := newViper(cfg)

		newV.SetConfigFile(f)
		err = newV.ReadInConfig()
		if err != nil {
			if isNotFoundErr(err) {
				cfg.Logger.Debug("no config file found, using defaults")
			} else {
				return nil, fmt.Errorf("unable to load config: %w", err)
			}
		}

		all := v.AllSettings()
		incoming := newV.AllSettings()

		// merge configuration slices in priority order, so slices will have high priority entries first, and retain
		// existing entries instead of overwriting them
		err = mergo.Merge(&all, incoming, mergo.WithAppendSlice)
		if err != nil {
			return nil, err
		}

		// viper merge will overwrite same keys, we have appended slices to the previous config in the previous step
		err = v.MergeConfigMap(all)
		if err != nil {
			return nil, err
		}
	}

	// we had been setting config previously to a string, so keep this behavior for now;
	// viper seems to magically split this string if the target is a []string
	v.Set("config", strings.Join(files, ","))

	return v, nil
}

// newViper returns a configured new viper instance
func newViper(cfg Config) *viper.Viper {
	// EnvKeyReplacer allows for nested options to be specified via environment variables
	// e.g. pod.context = APPNAME_POD_CONTEXT
	var v = viper.NewWithOptions(viper.EnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_")))

	// load environment variables
	v.SetEnvPrefix(cfg.AppName)
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()
	return v
}

// mergeProfiles merges profile sections in the viper config map to appropriate locations in the top-level configuration
func mergeProfiles(cfg Config, v *viper.Viper) error {
	if len(cfg.Profiles) == 0 {
		return nil // no profiles requested
	}
	if cfg.ProfileKey == "" {
		return fmt.Errorf("invalid configuration: fangs.Config.ProfileKey not defined")
	}
	// merge all profiles in to main configuration locations, overwriting
	all := v.AllSettings()
	profiles, ok := all[cfg.ProfileKey].(map[string]any)
	if !ok || profiles == nil {
		return fmt.Errorf("'%v' not found in any configuration files", cfg.ProfileKey)
	}
	for _, profileName := range Flatten(cfg.Profiles...) {
		profileVals, ok := profiles[profileName].(map[string]any)
		if !ok || profileVals == nil {
			// profile not defined, consider this an error as the user explicitly requested it and probably mistyped
			return fmt.Errorf("profile not found in any configuration files: %v", profileName)
		}
		// overwrite same keys -- this is what we want for profile selection, the profiles will already have
		// appended values if the same profile was found in multiple config files
		err := mergo.Merge(&all, profileVals, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
		if err != nil {
			return err
		}
		// merge the incoming config, this should replace anything in the existing config with the new values
		err = v.MergeConfigMap(all)
		if err != nil {
			return err
		}
	}
	return nil
}

// configureViper loads the default configuration values into the viper instance,
// before the config values are read and parsed. the value _must_ be a pointer but
// may be a pointer to a pointer
//

func configureViper(cfg Config, vpr *viper.Viper, configuring []reflect.Type, visited set[reflect.Value], v reflect.Value, flags flagRefs, path []string) {
	if visited.contains(v) {
		return
	}
	visited.add(v)

	t := v.Type()
	if !isPtr(t) {
		panic(fmt.Sprintf("configureViper v must be a pointer, got: %#v", v))
	}

	// v is always a pointer
	ptr := v.Pointer()
	t = t.Elem()
	v = v.Elem()

	// might be a pointer value
	for isPtr(t) {
		t = t.Elem()
		v = v.Elem()
	}

	if !isStruct(t) {
		envVar := envVar(cfg.AppName, path...)
		path := strings.Join(path, ".")

		if flag, ok := flags[ptr]; ok {
			cfg.Logger.Tracef("binding env var w/flag: %s", envVar)
			err := vpr.BindPFlag(path, flag)
			if err != nil {
				cfg.Logger.Debugf("unable to bind flag: %s to %#v", path, flag)
			}
			return
		}

		cfg.Logger.Tracef("binding env var: %s", envVar)

		vpr.SetDefault(path, nil) // no default value actually needs to be set for Viper to read config values
		return
	}

	// for each field in the configuration struct, see if the field implements the defaultValueLoader interface and invoke it if it does
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !includeField(f) {
			continue
		}

		path := path
		if tag, ok := f.Tag.Lookup(cfg.TagName); ok {
			// handle ,squash mapstructure tags
			parts := strings.Split(tag, ",")
			tag = parts[0]
			if tag == "-" {
				continue
			}
			switch {
			case contains(parts, "squash"):
				// use the current path
			case tag == "":
				path = append(path, f.Name)
			default:
				path = append(path, tag)
			}
		} else {
			path = append(path, f.Name)
		}

		if !v.IsValid() {
			// v is an unitialized embedded struct pointer to an unexported type.
			// This is considered private, and we won't be able to set any values on it.
			// Skipping this to avoid a panic.
			continue
		}
		v := v.Field(i)

		t := f.Type
		fieldConfiguring := configuring
		if isPtr(t) && v.IsNil() {
			t = t.Elem()
			if isStruct(t) {
				// don't keep creating recursive
				if slices.Contains(fieldConfiguring, t) {
					continue
				}
				fieldConfiguring = append(fieldConfiguring, t)

				newV := reflect.New(t)
				// v.CanSet can be false if we're trying to set a field on a struct
				// embedded via pointer when the embedded struct is unexported
				if v.CanSet() {
					v.Set(newV)
				}
			}
		}

		configureViper(cfg, vpr, fieldConfiguring, visited, v.Addr(), flags, path)
	}
}

func postLoad(v reflect.Value) error {
	t := v.Type()

	for isPtr(t) {
		if v.IsNil() {
			return nil
		}

		if v.CanInterface() {
			obj := v.Interface()
			if p, ok := obj.(PostLoader); ok && !isPromotedMethod(obj, "PostLoad") {
				if err := p.PostLoad(); err != nil {
					return err
				}
			}
		}
		t = t.Elem()
		v = v.Elem()
	}

	switch {
	case isStruct(t):
		return postLoadStruct(v)
	case isSlice(t):
		return postLoadSlice(v)
	case isMap(t):
		return postLoadMap(v)
	}

	return nil
}

// postLoadStruct call recursively on struct fields
func postLoadStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if !includeField(f) {
			continue
		}

		v := v.Field(i)

		if isNil(v) {
			continue
		}

		for isPtr(v.Type()) {
			v = v.Elem()
		}

		if !v.CanAddr() {
			continue
		}

		if err := postLoad(v.Addr()); err != nil {
			return err
		}
	}
	return nil
}

// postLoadSlice call recursively on slice items
func postLoadSlice(v reflect.Value) error {
	for i := 0; i < v.Len(); i++ {
		v := v.Index(i)

		if isNil(v) {
			continue
		}

		for isPtr(v.Type()) {
			v = v.Elem()
		}

		if !v.CanAddr() {
			continue
		}

		if err := postLoad(v.Addr()); err != nil {
			return err
		}
	}
	return nil
}

// postLoadMap call recursively on map values
func postLoadMap(v reflect.Value) error {
	mapV := v
	i := v.MapRange()
	for i.Next() {
		v := i.Value()

		if isNil(v) {
			continue
		}

		for isPtr(v.Type()) {
			v = v.Elem()
		}

		if !v.CanAddr() {
			// unable to call .Addr() on struct map entries, so copy to a new instance and set on the map
			if isStruct(v.Type()) {
				newV := reflect.New(v.Type())
				newV.Elem().Set(v)
				if err := postLoad(newV); err != nil {
					return err
				}
				mapV.SetMapIndex(i.Key(), newV.Elem())
			}

			continue
		}

		if err := postLoad(v.Addr()); err != nil {
			return err
		}
	}
	return nil
}

type flagRefs map[uintptr]*pflag.Flag

func commandFlagRefs(cmd *cobra.Command) flagRefs {
	return getFlagRefs(cmd.PersistentFlags(), cmd.Flags())
}

func getFlagRefs(flagSets ...*pflag.FlagSet) flagRefs {
	refs := flagRefs{}
	for _, flags := range flagSets {
		flags.VisitAll(func(flag *pflag.Flag) {
			refs[getFlagRef(flag)] = flag
		})
	}
	return refs
}

func getFlagRef(flag *pflag.Flag) uintptr {
	v := reflect.ValueOf(flag.Value)

	// check for struct types like stringArrayValue
	if isPtr(v.Type()) {
		vf := v.Elem()
		vt := vf.Type()
		if isStruct(vt) {
			if _, ok := vt.FieldByName("value"); ok {
				vf = vf.FieldByName("value")
				if vf.IsValid() {
					v = vf
				}
			}
		}
	}
	return v.Pointer()
}

func upperFirst(p string) string {
	if len(p) < 2 {
		return strings.ToUpper(p)
	}
	return strings.ToUpper(p[0:1]) + p[1:]
}

func isPtr(typ reflect.Type) bool {
	return typ.Kind() == reflect.Ptr
}

func isStruct(typ reflect.Type) bool {
	return typ.Kind() == reflect.Struct
}

func isSlice(typ reflect.Type) bool {
	return typ.Kind() == reflect.Slice
}

func isMap(typ reflect.Type) bool {
	return typ.Kind() == reflect.Map
}

func isNil(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Type().Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
	}
	return false
}

// isNotFoundErr returns true if the error is a viper.ConfigFileNotFoundError
func isNotFoundErr(err error) bool {
	var notFound *viper.ConfigFileNotFoundError
	return err != nil && errors.As(err, &notFound)
}

// includeField determines whether to include or skip a field when processing the application's nested configuration load.
// fields that are processed include: public/exported fields, embedded structs (not pointer private/unexported embedding)
func includeField(f reflect.StructField) bool {
	return (f.Anonymous && !isPtr(f.Type)) || f.IsExported()
}

// rootAt returns a new object with the provided the configuration object nested at the given path
func rootAt(cfg Config, configuration any, path string) any {
	t := reflect.TypeOf(configuration)
	config := reflect.StructOf([]reflect.StructField{{
		Name: upperFirst(path),
		Type: t,
		Tag:  reflect.StructTag(fmt.Sprintf(`%s:"%s"`, cfg.TagName, path)),
	}})

	value := reflect.New(config)
	value.Elem().Field(0).Set(reflect.ValueOf(configuration))
	return value.Interface()
}

type set[T comparable] map[T]struct{}

func (s set[T]) add(v T) {
	s[v] = struct{}{}
}

func (s set[T]) contains(v T) bool {
	_, ok := s[v]
	return ok
}
