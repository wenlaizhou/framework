package framework

import "fmt"

var apiList = make(map[string]ApiDesc)

type ApiDesc struct {
	Path   string
	Params []ApiParam
	Result ApiResult
	Desc   string
	Method string
}

type ApiParam struct {
	Name string
	Desc string
	Type string
}

type ApiResult struct {
	Type string
	Desc string
}

func apiProcessor(context Context) {
	context.ApiResponse(0, "", apiList)
	return
}

func init() {
	RegisterHandler("api", apiProcessor)
}

//参数列表,
//返回值说明,
//接口描述,
//异常说明
func (this *Server) RegisterApi(
	path string,
	method string,
	params []ApiParam,
	result ApiResult,
	desc string,
	handler func(context Context) (code int, message string, data interface{})) {

	if len(path) < 0 || handler == nil {
		return
	}

	this.RegisterHandler(path, func(context Context) {
		if context.GetMethod() != method {
			context.Error(405, fmt.Sprintf(StatusErrorTemp, "Method Not Allowed"))
			return
		}
		context.ApiResponse(handler(context))
	})

	apiList[path] = ApiDesc{
		Path:   path,
		Method: method,
		Desc:   desc,
		Params: params,
		Result: result,
	}

	return
}

func RegisterApi(path string,
	method string,
	params []ApiParam,
	result ApiResult,
	desc string,
	handler func(context Context) (code int, message string, data interface{})) {
	globalServer.RegisterApi(
		path,
		method,
		params,
		result,
		desc,
		handler)
}

//直接注册带界面api:
//1. 使用accept进行界面或json区分
//2. 调用权限区分
