package framework

import (
	"net/http"
	"regexp"
	"html/template"
	"strings"
	"log"
	"fmt"
	"path/filepath"
	"os"
	"sync"
	"errors"
	"encoding/json"
	"io/ioutil"
)

type Server struct {
	Host        string
	Port        int
	baseTpl     *template.Template
	pathNodes   map[string]pathProcessor
	index       pathProcessor
	hasIndex    bool
	CrossDomain bool
	status      int
	filter      []filterProcessor
	sync.RWMutex
}

var globalServer = NewServer("", 0)

func StartServer(host string, port int) {
	globalServer.Lock()
	globalServer.Host = host
	globalServer.Port = port
	globalServer.Unlock()
	globalServer.Start()
}

func GetGlobalServer() Server {
	return globalServer
}

func NewServer(host string, port int) Server {
	srv := Server{
		Host:        host,
		Port:        port,
		CrossDomain: false,
		hasIndex:    false,
	}

	srv.pathNodes = make(map[string]pathProcessor)
	return srv
}

func (this *Server) GetStatus() int {
	this.RLock()
	defer this.RUnlock()
	return this.status
}

func (this *Server) Start() {
	this.Lock()
	if this.status != 0 {
		this.Unlock()
		return
	}
	this.status = 1
	this.Unlock()
	hostStr := fmt.Sprintf("%s:%d", this.Host, this.Port)
	log.Println("server start " + hostStr)
	http.ListenAndServe(hostStr, this)
}

func (this *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(w, r)
	ctx.tpl = this.baseTpl
	if this.CrossDomain {
		ctx.SetHeader(AccessControlAllowOrigin, "*")
		ctx.SetHeader(AccessControlAllowMethods, METHODS)
		ctx.SetHeader(AccessControlAllowHeaders, "*")
		if strings.ToUpper(ctx.GetMethod()) == OPTIONS {
			ctx.Code(202)
			return
		}
	}

	for _, filterNode := range this.filter {
		if filterNode.pathReg.MatchString(r.RequestURI) {
			if !filterNode.handler(ctx) {
				return
			}
		}
	}

	if this.hasIndex && r.RequestURI == "/" {
		this.index.handler(ctx)
		return
	}

	for _, pathNode := range this.pathNodes {
		if pathNode.pathReg.MatchString(r.URL.Path) {
			pathParams := pathNode.pathReg.FindAllStringSubmatch(r.RequestURI, 10) //最多10个路径参数
			if len(pathParams) > 0 && len(pathParams[0]) > 0 {
				for i, pathParam := range pathParams[0][1:] {
					if len(pathNode.params) < i+1 {
						break
					}
					ctx.pathParams[pathNode.params[i]] = pathParam
				}
			}
			pathNode.handler(ctx)
			return
		}
	}
	ctx.Error(StatusNotFound, StatusNotFoundView)
	return
}

func (this *Server) Static(path string) {
	if !strings.HasSuffix(path, "/") {
		path = fmt.Sprintf("%s/", path)
	}
	this.RegisterHandler(path, StaticProcessor)
}

func (this *Server) RegisterIndex(handler func(Context)) {
	this.Lock()
	defer this.Unlock()
	this.hasIndex = true
	this.index = pathProcessor{
		handler: handler,
	}
}

func RegisterIndex(handler func(Context)) {
	globalServer.RegisterIndex(handler)
}

func RegisterStatic(path string) {
	globalServer.Static(path)
}

func (this *Server) RegisterTemplate(filePath string) {
	this.Lock()
	defer this.Unlock()
	this.baseTpl, _ = includeTemplate(this.baseTpl, ".html", []string{filePath}...)
	log.Println(this.baseTpl.DefinedTemplates())
}

func RegisterTemplate(filePath string) {
	globalServer.RegisterTemplate(filePath)
}

func (this *Server) TemplateFunc(name string, function interface{}) {
	this.Lock()
	defer this.Unlock()
	this.baseTpl.Funcs(template.FuncMap{
		name: function,}, )
}

func TemplateFunc(name string, function interface{}) {
	globalServer.TemplateFunc(name, function)
}

func includeTemplate(tpl *template.Template, suffix string, filePaths ... string) (*template.Template, error) {
	fileList := make([]string, 0)
	for _, filePath := range filePaths {
		info, err := os.Stat(filePath)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		if info.IsDir() {
			filepath.Walk(filePath, func(path string, innerInfo os.FileInfo, err error) error {
				if !innerInfo.IsDir() {
					//后缀名过滤
					if filepath.Ext(innerInfo.Name()) == suffix {
						fileList = append(fileList, path)
					}
				}
				return nil
			})
		} else {
			if filepath.Ext(filePath) == suffix {
				fileList = append(fileList, filePath)
			}
		}
	}
	log.Println("获取模板文件列表")
	log.Println(strings.Join(fileList, ","))
	if tpl == nil {
		return template.ParseFiles(fileList...)
	}
	return tpl.ParseFiles(fileList...)
}

var pathParamReg, _ = regexp.Compile("\\{(.*?)\\}")

func RegisterHandler(path string, handler func(Context)) {
	globalServer.RegisterHandler(path, handler)
}

/*
注册handler时, 有prefix扰乱
 */
func (this *Server) RegisterHandler(path string, handler func(Context)) {
	this.Lock()
	defer this.Unlock()
	if len(path) <= 0 {
		return
	}
	if handler == nil {
		return
	}
	if strings.HasSuffix(path, "/") {
		path = fmt.Sprintf("%s.*", path)
	} else {
		path = fmt.Sprintf("%s$", path)
	}

	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	paramMather := pathParamReg.FindAllStringSubmatch(path, -1)

	var params []string

	for _, param := range paramMather {
		params = append(params, param[1])
		path = strings.Replace(path,
			param[0], "(.*)", -1)
	}

	pathReg, err := regexp.Compile(path)
	log.Printf(fmt.Sprintf("注册handler: %s", path))
	if !ProcessError(err) {
		this.pathNodes[path] = pathProcessor{
			pathReg: pathReg,
			handler: handler,
			params:  params,
		}
	}
}

type triNode struct {
	path    string
	pathReg regexp.Regexp
	childs  []triNode
	data    *interface{}
}

type pathProcessor struct {
	pathReg *regexp.Regexp
	params  []string
	handler func(Context)
}

func StaticProcessor(ctx Context) {
	//解决?参数问题
	http.ServeFile(ctx.Response, ctx.Request, ctx.Request.URL.Path[1:])
}

// 错误处理
//
// return true 错误发生
//
// false 无错误
func ProcessError(err error) bool {
	if err != nil {
		log.Println(err)
		return true
	}
	return false
}

type Context struct {
	Request    *http.Request
	Response   http.ResponseWriter
	body       []byte
	tpl        *template.Template
	pathParams map[string]string
	writeable  bool
	sync.RWMutex
}

/*
获取路径参数, /{参数名称}
 */
func (this *Context) GetPathParam(key string) string {
	value, ok := this.pathParams[key]
	if ok {
		return value
	}
	return ""
}

func (this *Context) GetBody() ([]byte) {
	this.Lock()
	defer this.Unlock()
	if len(this.body) > 0 {
		return this.body
	}
	data, err := ioutil.ReadAll(this.Request.Body)
	this.body = data
	if err == nil && len(data) > 0 {
		this.body = data
		return this.body
	}
	return nil
}

func (this *Context) GetJSON() (map[string]interface{}, error) {
	res := make(map[string]interface{})
	if len(this.GetBody()) > 0 {
		err := json.Unmarshal(this.GetBody(), &res)
		return res, err
	}
	return res, nil
}

/*
获取query参数
 */
func (this *Context) GetQueryParam(key string) string {
	return this.Request.URL.Query().Get(key)
}

func (this *Context) WriteJSON(data interface{}) error {
	res, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = this.OK(ApplicationJson, res)
	return err
}

func (this *Context) GetContentType() string {
	return this.Request.Header.Get(ContentType)
}

func (this *Context) GetHeader(key string) string {
	return this.Request.Header.Get(key)
}

func (this *Context) GetCookie(key string) string {
	cook, err := this.Request.Cookie(key)
	if err != nil {
		return ""
	}
	return cook.Value
}

func (this *Context) SetCookie(c *http.Cookie) {
	http.SetCookie(this.Response, c)
}

func (this *Context) SessionSet(key string, value interface{}) {
	getSession(*this).Set(key, value)
}

func (this *Context) SessionGet(key string) interface{} {
	return getSession(*this).Get(key)
}

// 302跳转
func (this *Context) Redirect(path string) error {
	this.Lock()
	defer this.Unlock()
	if !this.writeable {
		return errors.New("禁止重复写入response")
	}
	this.writeable = false
	http.Redirect(this.Response, this.Request, path, http.StatusFound)
	return nil
}

func (this *Context) OK(contentType string, content []byte) error {
	this.Lock()
	defer this.Unlock()
	if !this.writeable {
		return errors.New("禁止重复写入response")
	}
	this.writeable = false
	if len(contentType) > 0 {
		this.SetHeader(ContentType, contentType)
	}
	this.SetHeader("server", "framework")
	_, err := this.Response.Write(content)
	return err
}

func (this *Context) Code(static int) error {
	this.Lock()
	defer this.Unlock()
	if !this.writeable {
		return errors.New("禁止重复写入response")
	}
	this.writeable = false
	this.SetHeader("server", "framework")
	this.Response.WriteHeader(static)
	return nil
}

func (this *Context) Error(static int, htmlStr string) error {
	this.Lock()
	defer this.Unlock()
	if !this.writeable {
		return errors.New("禁止重复写入response")
	}
	this.writeable = false
	this.SetHeader("server", "framework")
	this.SetHeader(ContentType, Html)
	this.Response.WriteHeader(static)
	this.Response.Write([]byte(htmlStr))
	return nil
}

func (this *Context) RenderTemplate(name string, model interface{}) error {
	if this.tpl != nil {
		return this.tpl.ExecuteTemplate(this.Response, name, model)
	}
	return errors.New("template 不存在")
}

func (this *Context) RenderTemplateKV(name string, kvs ...interface{}) error {
	if this.tpl == nil {
		return errors.New("template 不存在")
	}
	model := make(map[string]interface{})
	for i := 0; i < len(kvs); i += 2 {
		if v, ok := kvs[i].(string); ok {
			model[v] = kvs[i+1]
		}
	}
	return this.tpl.ExecuteTemplate(this.Response, name, model)
}

func (this *Context) SetHeader(key string, value string) {
	this.Response.Header().Set(key, value)
}

func (this *Context) DelHeader(key string) {
	this.Response.Header().Del(key)
}

func newContext(w http.ResponseWriter, r *http.Request) Context {
	return Context{
		writeable:  true,
		Response:   w,
		Request:    r,
		pathParams: make(map[string]string),
	}
}

func (this *Context) GetMethod() string {
	return this.Request.Method
}

func (this *Context) JSON(jsonStr string) error {
	err := this.OK(ApplicationJson, []byte(jsonStr))
	return err
}

func (this *Context) ApiResponse(code int, message string, data interface{}) error {
	model := make(map[string]interface{})
	model["code"] = code
	model["message"] = message
	model["data"] = data
	res, err := json.Marshal(model)
	if ProcessError(err) {
		return err
	}
	err = this.OK(ApplicationJson, res)
	return err
}

func (this *Context) RemoteAddr() string {
	return this.Request.RemoteAddr
}

/*
http文件服务
 */
func (this *Context) ServeFile(filePath string) {
	this.Lock()
	defer this.Unlock()
	if !this.writeable {
		return
	}
	http.ServeFile(this.Response, this.Request, filePath)
	this.writeable = false
}
