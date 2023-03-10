package customfuncs

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/dop251/goja"
	"github.com/jf-tech/go-corelib/caches"

	"github.com/Perachi0405/ownediparse/idr"
	"github.com/Perachi0405/ownediparse/transformctx"
)

const (
	argNameNode = "_node"
)

// JSProgramCache caches *goja.Program. A *goja.Program is compiled javascript, and it can be used
// across multiple goroutines and across different *goja.Runtime. If default loading cache capacity
// is not desirable, change JSProgramCache to a loading cache with a different capacity at package
// init time. Be mindful this will be shared across all use cases inside your process.
var JSProgramCache *caches.LoadingCache

// jsRuntimePool caches *goja.Runtime whose creation is expensive such that we want to have a pool
// of them to amortize the initialization cost. However, a *goja.Runtime cannot be used by two/more
// javascript's at the same time, thus the use of sync.Pool. Not user customizable.
var jsRuntimePool sync.Pool

// NodeToJSONCache caches *idr.Node to JSON translations.
var NodeToJSONCache *caches.LoadingCache

// For debugging/testing purpose, so we can easily disable all the caches. But not exported. We always
// want caching in production.
var disableCaching = false

//creating the cache.
func resetCaches() {
	fmt.Println("resetCaches")
	JSProgramCache = caches.NewLoadingCache()
	fmt.Println("JSProgramCache", reflect.TypeOf(JSProgramCache))
	jsRuntimePool = sync.Pool{ //A Pool is a set of temporary objects that may be individually saved and retrieved.
		New: func() interface{} {
			return goja.New() //Run JavaScript and get the result value.
		},
	}
	NodeToJSONCache = caches.NewLoadingCache() //creating new cache
	fmt.Println("NodeToJSONCache", NodeToJSONCache)
}

func init() {
	resetCaches()
}

//logs not found
func getProgram(js string) (*goja.Program, error) {
	fmt.Println("inside getProgram")
	if disableCaching {
		return goja.Compile("", js, false)
	}
	p, err := JSProgramCache.Get(js, func(interface{}) (interface{}, error) {
		return goja.Compile("", js, false)
	})
	if err != nil {
		return nil, err
	}
	return p.(*goja.Program), nil
}

func getNodeJSON(n *idr.Node) string {
	fmt.Println("Node check getNodeJSON")
	if disableCaching {
		return idr.JSONify2(n)
	}
	j, _ := NodeToJSONCache.Get(n.ID, func(interface{}) (interface{}, error) {
		return idr.JSONify2(n), nil
	})
	return j.(string)
}

//log not found
func execProgram(program *goja.Program, args map[string]interface{}) (goja.Value, error) {
	fmt.Println("Inside execProgram")
	var vm *goja.Runtime
	var poolObj interface{}
	if disableCaching {
		vm = goja.New()
	} else {
		poolObj = jsRuntimePool.Get()
		vm = poolObj.(*goja.Runtime)
	}
	defer func() {
		if vm != nil {
			// wipe out all the args in prep for next exec.
			for arg := range args {
				_ = vm.GlobalObject().Delete(arg)
			}
		}
		if poolObj != nil {
			jsRuntimePool.Put(poolObj)
		}
	}()
	for arg, val := range args {
		vm.Set(arg, val)
	}
	return vm.RunProgram(program)
}

//log not found
// JavaScriptWithContext is a custom_func that runs a javascript with optional arguments and
// with contextual '_node' JSON, if idr.Node is provided.
func JavaScriptWithContext(_ *transformctx.Ctx, n *idr.Node, js string, args ...interface{}) (interface{}, error) {
	fmt.Println("Inside JavaScriptWithContext ")
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("number of args must be even, but got %d", len(args))
	}
	program, err := getProgram(js)
	if err != nil {
		return nil, fmt.Errorf("invalid javascript: %s", err.Error())
	}
	vmArgs := make(map[string]interface{})
	for i := 0; i < len(args)/2; i++ {
		vmArgs[args[i*2].(string)] = args[i*2+1]
	}
	if n != nil {
		vmArgs[argNameNode] = getNodeJSON(n)
	}
	v, err := execProgram(program, vmArgs)
	if err != nil {
		return nil, err
	}
	switch {
	case goja.IsNaN(v), goja.IsInfinity(v), goja.IsNull(v), goja.IsUndefined(v):
		return nil, fmt.Errorf("result is %s", v.String())
	default:
		return v.Export(), nil
	}
}

//Log not found
// JavaScript is a custom_func that runs a javascript with optional arguments and without contextual
// '_node' JSON provided.
func JavaScript(ctx *transformctx.Ctx, js string, args ...interface{}) (interface{}, error) {
	fmt.Println("Inside the Javascript fun")
	fmt.Println("Inside Javascript ctx", ctx)
	fmt.Println("Inside Javascript args", args)
	return JavaScriptWithContext(ctx, nil, js, args...)
}
