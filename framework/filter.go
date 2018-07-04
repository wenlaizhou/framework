package framework

import (
	"regexp"
	"strings"
)

/*
注册过滤器

handle : return false 拦截请求
 */
func (this *Server) RegisterFilter(path string, handle func(Context) bool) {
	this.Lock()
	defer this.Unlock()
	if len(path) <= 0 {
		return
	}
	if strings.HasSuffix(path, "/") {
		path = path + ".*"
	}
	this.filter = append(this.filter, filterProcessor{
		handler: handle,
		pathReg: regexp.MustCompile(path),
	})

}

/*
注册过滤器

handle : return false 拦截请求
 */
func RegisterFilter(path string, handle func(Context) bool) {
	globalServer.RegisterFilter(path, handle)
}

type filterProcessor struct {
	pathReg *regexp.Regexp
	params  []string
	handler func(Context) bool
}
