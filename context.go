package hhttp

import (
	"net/http"
	"sync"
)

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (params Params) Get(key string) (string, bool) {
	for _, param := range params {
		if key == param.Key {
			return param.Value, true
		}
	}
	return "", false
}

func (params *Params) Set(key string, value string) {
	*params = append(*params, Param{key, value})
}

func (params *Params) Reset() {
	*params = (*params)[:0]
}

var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Context)
	},
}

func getContext() *Context {
	return ctxPool.Get().(*Context)
}

func putContext(ctx *Context) {
	ctxPool.Put(ctx)
}

type Context struct {
	Params  Params
	Request *http.Request
	Writer  http.ResponseWriter
}
