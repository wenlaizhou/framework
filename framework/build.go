package framework

import "net/http"

//todo 将资源文件编译进代码中

var content map[string][]byte

type BuildFileSystem struct {
	http.FileSystem
}

func (this *BuildFileSystem) Open(name string) (http.File, error) {
	return nil, nil
}

func BuildStatic(staticPath string) {

}

func BuildTemplate(templatePath string) {

}
