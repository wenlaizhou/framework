package rest

import (
	"github.com/wenlaizhou/framework/framework"
	"io/ioutil"
	"net/http"
	"strings"
	"log"
)

type ConstConf struct {
	Path  string
	Const string
}

type FileConf struct {
	Path        string
	ContentType string
	File        string
}

func InitConstConf(confPath string) {
	doc := framework.LoadXml(confPath)

	elements := doc.FindElements("//constApi")

	for _, ele := range elements {
		conf := new(ConstConf)

		conf.Path = ele.SelectAttrValue("path", "")
		conf.Const = strings.TrimSpace(ele.FindElement(".//const").Text())

		registerConstConf(*conf)
	}

}

func registerConstConf(conf ConstConf) {
	if len(conf.Path) <= 0 {
		log.Printf("Const-Api 注册失败 : %#v", conf)
		return
	}
	framework.RegisterHandler(conf.Path,
		func(context framework.Context) {
			context.ApiResponse(0, "", conf.Const)
		})
}

func ProcessFileConf(conf FileConf) {
	framework.RegisterHandler(conf.Path,
		func(context framework.Context) {
			if framework.Exists(conf.File) {
				data, err := ioutil.ReadFile(conf.File)
				if err != nil {
					context.Code(404, "")
					return
				}
				if len(conf.ContentType) > 0 {
					context.OK(conf.ContentType, data)
				} else {
					context.OK(http.DetectContentType(data), data)
				}
				return
			} else {
				context.Code(404, "")
			}
		})
}
