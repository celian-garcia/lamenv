// Package lamenv is proposing a way to unmarshall the environment variable in a golang structure.
//
// Source code and other details for the project are available at GitHub:
//
//   https://github.com/Nexucis/lamenv
//
package lamenv

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var defaultTagSupported = []string{
	"yaml", "json", "mapstructure",
}

// Unmarshal is looking at the object to guess which environment variable is matching.
//
// Maps and pointers (to a struct, string, int, etc) are accepted as object.
// If an internal pointer within a struct is not initialized,
// the lamenv package will initialize it if necessary for unmarshalling the provided data.
// The object parameter must not be nil.
// The parts can be used to inject a prefix of the environment variable
//
// Struct fields are only unmarshalled if they are exported (have an
// upper case first letter), and are unmarshalled using the field name
// uppercased as the default key. Custom keys can be defined via the
// "json", "yaml" and "mapstructure" name in the field tag.
// If multiple tag name are defined, "json" is considered at first, then "yaml" and finally "mapstructure".
//
// Note: When using a map, it's possible for the Unmarshal method to fail because it's finding multiple way to unmarshal
// the same environment variable for different field in the struct (that could be at different depth).
// It's usually because when using a map, the method has to guess which key to use to unmarshal the environment variable.
// And sometimes, it's possible there are severals keys found.
//
// Example of how to use it with the following environment variables available:
//    MY_PREFIX_A = 1
//    MY_PREFIX_B = 2
//
//    type T struct {
//    	F int `json:"a,omitempty"`
//    	B int
//    }
//    var t T
//    lamenv.Unmarshal(&t, []string{"MY_PREFIX"})
func Unmarshal(object interface{}, parts []string) error {
	return New().Unmarshal(object, parts)
}

// Lamenv is the exported struct of the package that can be used to fine-tune the way to unmarshall the different struct.
type Lamenv struct {
	// TagSupports is a list of tag like "yaml", "json"
	// that the code will look at it to know the name of the field
	TagSupports []string
	// env is the map that is representing the list of the environment variable visited
	// The key is the name of the variable.
	// The value is not important, since once the variable would be used, then the key will be removed
	// It will be useful when a map is involved in order to not parse every possible variable
	// but only the one that are still not used.
	env map[string]bool
}

// New is the method to use to initialize the struct Lamenv.
// The struct can then be fine tuned using the appropriate exported method.
func New() *Lamenv {
	env := make(map[string]bool)
	for _, e := range os.Environ() {
		envSplit := strings.Split(e, "=")
		if len(envSplit) != 2 {
			continue
		}
		env[envSplit[0]] = true
	}
	return &Lamenv{
		TagSupports: []string{
			"yaml", "json", "mapstructure",
		},
		env: env,
	}
}

// Unmarshal reads the object to guess and find the appropriate environment variable to use for the decoding.
// Once the environment variable matching the field looked is found, it will unmarshall the value and the set the field with it.
func (l *Lamenv) Unmarshal(object interface{}, parts []string) error {
	return l.decode(reflect.ValueOf(object), parts)
}

// AddTagSupport modify the current tag list supported by adding the one passed as a parameter.
// If you prefer to override the default tag list supported by Lamenv, use the method OverrideTagSupport instead.
func (l *Lamenv) AddTagSupport(tags ...string) *Lamenv {
	for _, tag := range tags {
		l.TagSupports = append(l.TagSupports, tag)
	}
	return l
}

// OverrideTagSupport overrides the current tag list supported by the one passed as a parameter.
// If you prefer to add new tag supported instead of overriding the current list, use the method AddTagSupport instead.
func (l *Lamenv) OverrideTagSupport(tags ...string) *Lamenv {
	l.TagSupports = tags
	return l
}

func (l *Lamenv) decode(conf reflect.Value, parts []string) error {
	v := conf
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// if the pointer is not initialized, then accessing to its element will return `reflect.invalid`
			// So we have to create a new instance of the pointer first
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		if err := l.decodeMap(v, parts); err != nil {
			return err
		}
	case reflect.Slice:
		if err := l.decodeSlice(v, parts); err != nil {
			return err
		}
	case reflect.Struct:
		if err := l.decodeStruct(v, parts); err != nil {
			return err
		}
	default:
		if variable, input, ok := lookupEnv(parts); ok {
			// remove the variable to avoid to reuse it later
			delete(l.env, variable)
			return l.decodeNative(v, input)
		}
	}
	return nil
}

func (l *Lamenv) decodeNative(v reflect.Value, input string) error {
	switch v.Kind() {
	case reflect.String:
		l.decodeString(v, input)
	case reflect.Bool:
		if err := l.decodeBool(v, input); err != nil {
			return err
		}
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		if err := l.decodeInt(v, input); err != nil {
			return err
		}
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		if err := l.decodeUInt(v, input); err != nil {
			return err
		}
	case reflect.Float32,
		reflect.Float64:
		if err := l.decodeFloat(v, input); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) decodeString(v reflect.Value, input string) {
	v.SetString(input)
}

func (l *Lamenv) decodeBool(v reflect.Value, input string) error {
	b, err := strconv.ParseBool(strings.TrimSpace(input))
	if err != nil {
		return err
	}
	v.SetBool(b)
	return nil
}

func (l *Lamenv) decodeInt(v reflect.Value, input string) error {
	i, err := strconv.ParseInt(strings.TrimSpace(input), 10, 0)
	if err != nil {
		return err
	}
	v.SetInt(i)
	return nil
}

func (l *Lamenv) decodeUInt(v reflect.Value, input string) error {
	i, err := strconv.ParseUint(strings.TrimSpace(input), 10, 0)
	if err != nil {
		return err
	}
	v.SetUint(i)
	return nil
}

func (l *Lamenv) decodeFloat(v reflect.Value, input string) error {
	i, err := strconv.ParseFloat(strings.TrimSpace(input), 10)
	if err != nil {
		return err
	}
	v.SetFloat(i)
	return nil
}

// decodeSlice will support ony one syntax which is:
//        <PREFIX>_<SLICE_INDEX>(_<SUFFIX>)?
// This syntax is the only one that is able to manage smoothly every existing type in Golang and it is a determinist syntax.
func (l *Lamenv) decodeSlice(v reflect.Value, parts []string) error {
	sliceType := v.Type().Elem()
	// While we are able to find an environment variable that is starting by <PREFIX>_<SLICE_INDEX>
	// then it will create a new item in a slice and will use the next recursive loop to set it.
	i := 0
	for ok := contains(append(parts, strconv.Itoa(i))); ok; ok = contains(append(parts, strconv.Itoa(i))) {
		// create a new item and pass it to the method decode to be able to "decode" its value
		tmp := reflect.Indirect(reflect.New(sliceType))
		if err := l.decode(tmp, append(parts, strconv.Itoa(i))); err != nil {
			return err
		}
		v.Set(reflect.Append(v, tmp))
		i++
	}
	return nil
}

func (l *Lamenv) decodeStruct(v reflect.Value, parts []string) error {
	for i := 0; i < v.NumField(); i++ {
		attr := v.Field(i)
		if !attr.CanSet() {
			// the field is not exported, so we won't be able to change its value.
			continue
		}
		attrField := v.Type().Field(i)
		attrName, ok := l.lookupTag(attrField.Tag)
		if ok {
			if attrName == "-" {
				continue
			}
			if attrName == ",squash" || attrName == ",inline" {
				if err := l.decode(attr, parts); err != nil {
					return err
				}
				continue
			}
			if strings.Contains(attrName, "omitempty") {
				// Here we only have to check if there is one environment variable that is starting by the current parts
				// It's not necessary accurate if you have one field that is a prefix of another field.
				// But it's not really a big deal since it will just loop another time for nothing and could eventually initialize the field. But this case will not occur so often.
				// To be more accurate, we would have to check the type of the field, because if it's a native type, then we will have to check if the parts are matching an environment variable.
				// If it's a struct or an array or a map, then we will have to check if there is at least one variable starting by the parts + "_" (which would remove the possibility of having a field being a prefix of another one)
				// So it's simpler like that. Let's see if I'm wrong or not.
				if !contains(append(parts, attrName)) {
					continue
				}
			}
		} else {
			attrName = attrField.Name
		}
		if err := l.decode(attr, append(parts, attrName)); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lamenv) decodeMap(v reflect.Value, parts []string) error {
	keyType := v.Type().Key()
	valueType := v.Type().Elem()
	if keyType.Kind() != reflect.String {
		return fmt.Errorf("unable to unmarshal a map with a key that is not a string")
	}
	if valueType.Kind() == reflect.Map {
		return fmt.Errorf("unable to unmarshal a map of a map, it's not a determinist datamodel")
	}
	valMap := v
	if v.IsNil() {
		mapType := reflect.MapOf(keyType, valueType)
		valMap = reflect.MakeMap(mapType)
	}
	// The main issue with the map when you are dealing with environment variable is to be able to find the key of the map
	// A way to achieve it is to take a look at the type of the value of the map.
	// It will be used to find every potential future parts, which will be then used as a variable suffix.
	// Like that we are able catch the key that would be in the middle of the prefix parts and the future parts

	// Let's create first the struct that would represent what is behind the value of the map
	parser := newRing(valueType, l.TagSupports)

	// then foreach environment variable:
	// 1. Remove the prefix parts
	// 2. Pass the remaining parts to the parser that would return the prefix to be used.
	for e := range l.env {
		variable := buildEnvVariable(parts)
		trimEnv := strings.TrimPrefix(e, variable+"_")
		if trimEnv == e {
			// TrimPrefix didn't remove anything, so that means, the environment variable doesn't start with the prefix parts
			continue
		}
		futureParts := strings.Split(trimEnv, "_")
		prefix, err := guessPrefix(futureParts, parser)
		if err != nil {
			return err
		}
		if len(prefix) == 0 {
			// no prefix find, let's move to the next environment
			continue
		}
		keyString := strings.ToLower(prefix)
		value := reflect.Indirect(reflect.New(valueType))
		if err := l.decode(value, append(parts, keyString)); err != nil {
			return err
		}
		key := reflect.Indirect(reflect.New(reflect.TypeOf("")))
		key.SetString(strings.TrimSpace(strings.ToLower(keyString)))
		valMap.SetMapIndex(key, value)
	}
	// Set the built up map to the value
	v.Set(valMap)
	return nil
}

func (l *Lamenv) lookupTag(tag reflect.StructTag) (string, bool) {
	return lookupTag(tag, l.TagSupports)
}

func contains(parts []string) bool {
	variable := buildEnvVariable(parts)
	for _, e := range os.Environ() {
		envSplit := strings.Split(e, "=")
		if len(envSplit) != 2 {
			continue
		}
		if strings.Contains(envSplit[0], variable) {
			return true
		}
	}
	return false
}

// lookupEnv is returning:
// 1. the name of the environment variable
// 2. the value of the environment variable
// 3. if the environment variable exists
func lookupEnv(parts []string) (string, string, bool) {
	variable := buildEnvVariable(parts)
	value, ok := os.LookupEnv(variable)
	return variable, value, ok
}

func lookupTag(tag reflect.StructTag, tagSupports []string) (string, bool) {
	for _, tagSupport := range tagSupports {
		if s, ok := tag.Lookup(tagSupport); ok {
			return s, ok
		}
	}
	return "", false
}

func buildEnvVariable(parts []string) string {
	newParts := make([]string, len(parts))
	for i, s := range parts {
		newParts[i] = strings.ToUpper(s)
	}
	return strings.Join(newParts, "_")
}
