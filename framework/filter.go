package framework

import (
	"regexp"
	"strings"
)

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

type filterProcessor struct {
	pathReg *regexp.Regexp
	params  []string
	handler func(Context) bool
}
