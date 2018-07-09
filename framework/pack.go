package framework

import (
	"os"
	"path/filepath"
)

func packDir(root string) {
	//静态文件编译到变量中
	//模板文件编译到变量中
	//将模板文本注册到全局数据体中

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {

		return nil
	})
}
