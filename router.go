package hhttp

import (
	"net/http"
	"regexp"
	"strings"
)

const defaultNumOfSegments = 16

var (
	varSegmentPattern1 = regexp.MustCompile("^:[^/]+$")
	varSegmentPattern2 = regexp.MustCompile("^{[^/]+}$")
)

type Handler func(ctx *Context)

func HandlerFunc(f http.HandlerFunc) Handler {
	return func(ctx *Context) {
		f(ctx.Writer, ctx.Request)
	}
}

func defaultNotFoundHandler(ctx *Context) {
	ctx.Writer.WriteHeader(http.StatusNotFound)
	ctx.Writer.Write([]byte("404 page not found"))
}

type Router struct {
	NotFoundHandler Handler

	middlewares []Handler
	routeTable  map[string][]*node
}

func NewRouter() *Router {
	return &Router{NotFoundHandler: defaultNotFoundHandler, routeTable: make(map[string][]*node)}
}

func (router *Router) Use(middlewares ...Handler) {
	router.middlewares = append(router.middlewares, middlewares...)
}

func (router *Router) AddRoute(method string, path string, handlers ...Handler) {
	segments := splitPath(path, make([]string, 0, defaultNumOfSegments))
	nodes := make([]*node, len(segments))
	for i, segment := range segments {
		switch {
		case varSegmentPattern1.MatchString(segment):
			nodes[i] = &node{isFixed: false, segment: segment[1:]}
		case varSegmentPattern2.MatchString(segment):
			nodes[i] = &node{isFixed: false, segment: segment[1 : len(segment)-1]}
		default:
			nodes[i] = &node{isFixed: true, segment: segment}
		}
	}
	for i := 0; i < len(nodes)-1; i++ {
		nodes[i].children = []*node{nodes[i+1]}
	}
	nodes[len(nodes)-1].handlers = handlers
	roots := router.routeTable[method]
	nodeExists := false
	for _, root := range roots {
		if merge(root, nodes[0]) {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		router.routeTable[method] = append(roots, nodes[0])
	}
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := getContext()
	defer putContext(ctx)
	ctx.Params.Reset()
	ctx.Request = r
	ctx.Writer = w
	roots := router.routeTable[r.Method]
	if len(roots) == 0 {
		router.NotFoundHandler(ctx)
		return
	}
	segments := splitPath(r.URL.Path, make([]string, 0, defaultNumOfSegments))
	path := findPath(make([]nodeWrapper, 0, defaultNumOfSegments), roots, segments)
	if len(path) == 0 || len(path[len(path)-1].handlers) == 0 {
		router.NotFoundHandler(ctx)
		return
	}
	for i, n := range path {
		if !n.isFixed {
			ctx.Params.Set(n.segment, segments[i])
		}
	}
	for _, middleware := range router.middlewares {
		middleware(ctx)
	}
	for _, handler := range path[len(path)-1].handlers {
		handler(ctx)
	}
}

func (router *Router) Group(path string) *RouterGroup {
	return &RouterGroup{r: router, path: strings.Trim(path, "/") + "/"}
}

type RouterGroup struct {
	r           *Router
	path        string
	middlewares []Handler
}

func (group *RouterGroup) Use(middlewares ...Handler) {
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup) AddRoute(method string, path string, handlers ...Handler) {
	group.r.AddRoute(method, group.path+strings.Trim(path, "/"), append(group.middlewares, handlers...)...)
}

type node struct {
	isFixed  bool
	segment  string
	children []*node
	handlers []Handler
}

func (n *node) equal(other *node) bool {
	return n.isFixed == other.isFixed && n.segment == other.segment
}

func (n *node) hasChild(c *node) int {
	for i, child := range n.children {
		if c.equal(child) {
			return i
		}
	}
	return -1
}

func merge(n1 *node, n2 *node) bool {
	if !n1.equal(n2) {
		return false
	}
	n1.handlers = append(n1.handlers, n2.handlers...)
	for _, child := range n2.children {
		if i := n1.hasChild(child); i != -1 {
			merge(n1.children[i], child)
		} else {
			n1.children = append(n1.children, child)
		}
	}
	return true
}

type nodeWrapper struct {
	*node
	nextChild int
}

func findPath(path []nodeWrapper, roots []*node, segments []string) []nodeWrapper {
	for _, root := range roots {
		if root.isFixed && root.segment != segments[0] {
			continue
		}
		path = append(path, nodeWrapper{root, 0})
	loop:
		for len(path) > 0 {
			if len(path) == len(segments) {
				return path
			}
			top := &path[len(path)-1]
			for top.nextChild < len(top.children) {
				child := top.children[top.nextChild]
				top.nextChild++
				if !child.isFixed || child.segment == segments[len(path)] {
					path = append(path, nodeWrapper{child, 0})
					continue loop
				}
			}
			path = path[:len(path)-1]
		}
	}
	return nil
}

func splitPath(path string, segments []string) []string {
	var (
		begin = 0
		end   = len(path) - 1
	)
	for begin < len(path) && path[begin] == '/' {
		begin++
	}
	for end >= 0 && path[end] == '/' {
		end--
	}
	if begin >= end {
		return append(segments, "")
	}
	start := begin
	for i := begin; i < end; i++ {
		if path[i] == '/' {
			segments = append(segments, path[start:i])
			start = i + 1
		}
	}
	return append(segments, path[start:end+1])
}
