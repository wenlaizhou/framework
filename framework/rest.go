package framework

var apiList = make(map[string]apiDesc)

type apiDesc struct {
	path       string
	paramDesc  string
	resultDesc string
	desc       string
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
	paramDesc string,
	resultDesc string,
	desc string,
	handler func(context Context)) {

	if len(path) < 0 || handler == nil {
		return
	}

	this.RegisterHandler(path, handler)

	apiList[path] = apiDesc{
		path:       path,
		desc:       desc,
		paramDesc:  paramDesc,
		resultDesc: resultDesc,
	}

	return
}

func RegisterApi(path string,
	paramDesc string,
	resultDesc string,
	desc string,
	handler func(context Context)) {
	globalServer.RegisterApi(
		path,
		paramDesc,
		resultDesc,
		desc,
		handler)
}
