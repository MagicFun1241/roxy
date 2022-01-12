package logger

import (
	"fmt"
	"github.com/dop251/goja"
	"github.com/yeqown/log"
	"reflect"
	"strconv"
)

type ConsoleModule struct {
	runtime *goja.Runtime
}

type formatOptions struct {
	PropName     *string
	StringQuotes bool
}

func formatValue(v goja.Value, opts formatOptions) string {
	if goja.IsNull(v) {
		return "null"
	}

	switch v.ExportType().Kind() {
	case reflect.String:
		if opts.StringQuotes {
			return fmt.Sprintf("\"%s\"", v.String())
		} else {
			return v.String()
		}
	case reflect.Int64:
		return strconv.Itoa(int(v.ToInteger()))
	case reflect.Bool:
		if v.ToBoolean() {
			return "true"
		} else {
			return "false"
		}
	default:
		if _, ok := goja.AssertFunction(v); ok {
			return fmt.Sprintf("[Function %s]", *opts.PropName)
		} else if o, ok := v.(*goja.Object); ok {
			t := ""

			if o.ClassName() == "Array" {
				if len(o.Keys()) == 0 {
					t = "[]"
					return t
				}

				t += "[ "
				for i, k := range o.Keys() {
					t += formatValue(o.Get(k), formatOptions{})

					if i != len(o.Keys())-1 {
						t += ", "
					}
				}
				t += " ]"

				return t
			}

			if len(o.Keys()) == 0 {
				t = "{}"
			} else {
				t += "{ "

				for i, k := range o.Keys() {
					prop := o.Get(k)
					t += k + ": " + formatValue(prop, formatOptions{PropName: &k, StringQuotes: true})

					if i != len(o.Keys())-1 {
						t += ", "
					}
				}

				t += " }"
			}
			return t
		} else {
			return "unknown"
		}
	}
}

func logRaw(call goja.FunctionCall) string {
	if len(call.Arguments) == 0 {
		return ""
	}

	var r = ""

	for i, arg := range call.Arguments {
		r += formatValue(arg, formatOptions{StringQuotes: false})

		if i != len(call.Arguments)-1 {
			r += " "
		}
	}

	return r
}

func (c *ConsoleModule) log(call goja.FunctionCall) goja.Value {
	log.Info(logRaw(call))
	return goja.Undefined()
}

func (c *ConsoleModule) error(call goja.FunctionCall) goja.Value {
	log.Error(logRaw(call))
	return goja.Undefined()
}

func Register(vm *goja.Runtime) {
	c := &ConsoleModule{runtime: vm}

	o := vm.NewObject()
	_ = o.Set("log", c.log)
	_ = o.Set("error", c.error)

	_ = vm.Set("console", o)
}
