package inject

import (
	"reflect"
	"github.com/hidevopsio/hiboot/pkg/log"
	"github.com/hidevopsio/hiboot/pkg/utils/reflector"
	"strings"
	"github.com/hidevopsio/hiboot/pkg/starter"
	"errors"
	"github.com/hidevopsio/hiboot/pkg/utils/mapstruct"
	"github.com/hidevopsio/hiboot/pkg/utils"
)


const (
	injectIdentifier = "inject"
	valueIdentifier = "value"
)

var (
	autoConfiguration starter.AutoConfiguration
	NotImplementedError = errors.New("interface is not implemented")
)

func init() {
	log.SetLevel(log.DebugLevel)
	autoConfiguration = starter.GetAutoConfiguration()
}

func replaceReferences(val string) string  {
	retVal := val
	systemConfig := autoConfiguration.Configuration(starter.System)

	matches := utils.GetMatches(val)
	if len(matches) != 0 {
		for _, m := range matches {
			//log.Debug(m[1])
			// default value

			vars := strings.SplitN(m[1], ".", -1)
			configName := vars[0]
			config := autoConfiguration.Configuration(configName)
			if config == nil && utils.GetReferenceValue(systemConfig, configName).IsValid() {
				config = systemConfig
			}
			if config != nil {
				retVal = utils.ReplaceStringVariables(val, config)
				if retVal != val {
					break
				}
			}
		}
	}
	return retVal
}

func parseInjectTag(tagValue string) map[string]interface{} {

	tags := make(map[string]interface{}) // ? map[string]string

	args := strings.Split(tagValue, ",")
	for _, v := range args {
		//log.Debug(v)
		kv := strings.Split(v, "=")
		if len(kv) == 2 {
			val := kv[1]
			// check if val contains reference or env
			// TODO: should lookup certain config instead of for loop
			replacedVal := replaceReferences(val)
			tags[kv[0]] = replacedVal
		}
	}

	return tags
}

func parseValue(valueTag string) string  {
	var retVal string
	if valueTag != "" {
		//log.Debug(valueTag)
		retVal = replaceReferences(valueTag)
	}
	return retVal
}

// IntoObject injects instance into the tagged field with `inject:"instanceName"`
func IntoObject(object reflect.Value) error {
    var err error
	instances := autoConfiguration.Instances()

	for _, f := range reflector.DeepFields(object.Type()) {
		//log.Debugf("parent: %v, name: %v, type: %v, tag: %v", object.Elem().Type(), f.Name, f.Type, f.Tag)
		// check if object has value field to be injected
		var injectedObject interface{}
		obj := reflector.Indirect(object)

		ft := f.Type
		if f.Type.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		valueTag, ok := f.Tag.Lookup(valueIdentifier)
		if ok {
			//log.Debugf("value tag: %v, %v", valueTag, ok)
			injectedObject = parseValue(valueTag)
		} else {
			injectTag, ok := f.Tag.Lookup(injectIdentifier)
			if ok {
				//log.Debugf("inject tag: %v, %v", injectTag, ok)
				instanceName := f.Name
				tags := parseInjectTag(injectTag)

				// first, find if object is already instantiated
				injectedObject = instances[instanceName]
				//log.Debugf("field kind: %v", ft.Kind())
				if injectedObject == nil {
					if ft.Kind() == reflect.Interface {
						return errors.New("interface " + ft.PkgPath() + "." + ft.Name() + " is not implemented in " + obj.Type().Name())
					} else {
						// if object is not exist, then instantiate new object
						// parse tag and instantiate filed
						o := reflect.New(ft)
						injectedObject = o.Interface()
						// inject field value
						if len(tags) != 0 {
							mapstruct.Decode(injectedObject, tags)
						}
						instances[instanceName] = injectedObject
					}
				}
			}
		}
		// set field object
		var fieldObj reflect.Value
		if obj.IsValid() {
			fieldObj = obj.FieldByName(f.Name)
		}
		if injectedObject != nil && fieldObj.CanSet() {
			fov := reflect.ValueOf(injectedObject)
			fieldObj.Set(fov)
			log.Debugf("Injected %v.(%v) into %v.%v", injectedObject, fov.Type(), obj.Type(), f.Name)
		}

		//log.Debugf("- kind: %v, %v, %v", obj.Kind(), object.Type(), fieldObj.Type())
		//log.Debugf("isValid: %v, canSet: %v", fieldObj.IsValid(), fieldObj.CanSet())
		if obj.Kind() == reflect.Struct && fieldObj.IsValid() && fieldObj.CanSet() {
			err = IntoObject(fieldObj)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

