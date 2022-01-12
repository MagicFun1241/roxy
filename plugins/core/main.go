package core

import (
	"github.com/dop251/goja"
	log "unknwon.dev/clog/v2"
)

type module struct {
	Runtime *goja.Runtime
}

type plugin struct {
	Permissions   []string
	OptionsSchema map[string]interface{}
}

var Middlewares = make(map[string]goja.Callable)
var Plugins = make(map[string]*plugin)

func (m *module) registerPlugin(call goja.FunctionCall) goja.Value {
	name := call.Argument(0).String()
	if Plugins[name] != nil {
		log.Error("Plugin '%s' already registered", name)
		return nil
	}

	options := call.Argument(1).ToObject(m.Runtime)
	optionsSchema := options.Get("optionsSchema").ToObject(m.Runtime)

	p := &plugin{
		OptionsSchema: map[string]interface{}{},
	}

	for _, k := range optionsSchema.Keys() {
		p.OptionsSchema[k] = optionsSchema.Get(k).Export()
	}

	Plugins[name] = p
	log.Info("Plugin '%s' loaded", name)
	return nil
}

func (m *module) registerMiddleware(call goja.FunctionCall) goja.Value {
	name := call.Argument(0).String()

	if Plugins[name] == nil {
		log.Error("Plugin '%s' is not registered", name)
		return nil
	}

	cb, ok := goja.AssertFunction(call.Argument(1))
	if !ok {
		log.Error("Can not register middleware in plugin '%s'", name)
		return nil
	}

	Middlewares[name] = cb
	log.Info("Middleware for plugin '%s' registered", name)
	return nil
}

func RegisterModule(vm *goja.Runtime) {
	m := &module{Runtime: vm}
	_ = vm.Set("register_plugin", m.registerPlugin)
	_ = vm.Set("register_middleware", m.registerMiddleware)
}
