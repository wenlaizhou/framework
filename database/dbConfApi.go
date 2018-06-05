package database

import (
	"github.com/wenlaizhou/framework/framework"
	"strings"
	"regexp"
	"strconv"
	"log"
)

type SqlApi struct {
	Result      int
	Path        string
	Transaction bool
	Sqls        []SqlConf
	Params      map[string]string
}

type SqlConf struct {
	HasSql    bool
	Type      string
	Table     string
	SqlOrigin string
	RParams   []SqlParam
	Params    []SqlParam
	Id        string
}

type SqlParam struct {
	Type  int
	Key   string
	Value interface{}
	//Id   string
}

const (
	Post   = 0 //${}
	Result = 1 //@{} result结果只能具有id类型
	Param  = 2
	//Replace = 2 //#{}
	//guid : {{guid}}
)

const (
	Insert = "insert"
	Select = "select"
	Update = "update"
	Delete = "delete"
)

const (
	Normal  = 0
	Combine = 1
)

//六种类型参数
//1: post sql参数
//2: result sql参数
//3: post replace参数
//4: result replace参数
//5: param sql参数
//6: param replace参数

var postReg = regexp.MustCompile("\\$\\{(.*?)\\}")
var resultReplaceReg = "#\\{%s\\.(.*?)\\}"
var resultReg = "$\\{%s\\.(.*?)\\}"
var replaceReg = regexp.MustCompile("#\\{(.*?)\\}")

func InitSqlConfApi(filePath string) {
	apiConf := framework.LoadXml(filePath)
	apiElements := apiConf.FindElements("//sqlApi")
	for _, apiEle := range apiElements {
		sqlIds := make([]string, 0)
		sqlApi := new(SqlApi)
		sqlApi.Transaction = apiEle.SelectAttrValue("transaction", "") == "true"
		sqlApi.Path = apiEle.SelectAttrValue("path", "")
		sqlApi.Sqls = make([]SqlConf, 0)
		sqlApi.Params = make(map[string]string)
		for _, paramEle := range apiEle.FindElements(".//param") {
			sqlApi.Params[paramEle.SelectAttrValue("key", "")] = paramEle.SelectAttrValue("value", "")
		}
		for i, sqlEle := range apiEle.FindElements(".//sql") {
			oneSql := new(SqlConf)
			oneSql.Table = sqlEle.SelectAttrValue("table", "")
			oneSql.Id = sqlEle.SelectAttrValue("id", strconv.Itoa(i))
			sqlIds = append(sqlIds, oneSql.Id)
			sqlStr := strings.TrimSpace(sqlEle.Text())
			if len(sqlStr) <= 0 {
				oneSql.HasSql = false
				oneSql.Type = sqlEle.SelectAttrValue("type", "")
				if !oneSql.HasSql && len(oneSql.Type) <= 0 {
					//配置错误
				}
			} else {
				//参数计算
				oneSql.SqlOrigin, oneSql.RParams, oneSql.Params = parseSql(oneSql.SqlOrigin)
			}
			sqlApi.Sqls = append(sqlApi.Sqls, *oneSql)
		}
		initSqlApi(*sqlApi)
	}

}

func initSqlApi(sqlApi SqlApi) {
	if len(sqlApi.Path) <= 0 {
		log.Printf("sqlApi注册失败 : %#v 没有服务路径", sqlApi)
		return
	}
	log.Printf("注册sql api服务: %#v", sqlApi)
	framework.RegisterHandler(sqlApi.Path,
		func(context framework.Context) {
			sqlApi := sqlApi
			//处理guid
			for k, v := range sqlApi.Params {
				if v == "{{guid}}" {
					sqlApi.Params[k] = framework.Guid()
				}
			}
			jsonData, err := context.GetJSON()
			if err != nil {
				jsonData = make(map[string]interface{})
			}
			session := DbApiInstance.GetEngine().NewSession()
			defer session.Close()
			if sqlApi.Transaction {
				session.Begin()
			}

			result := make([]map[string]string, 0)

			for _, sqlInstance := range sqlApi.Sqls {
				if sqlInstance.HasSql {
					oneSqlRes, err := exec(*session, sqlInstance, jsonData, sqlApi.Params)
					if err != nil {
						framework.ProcessError(session.Rollback())
						context.ApiResponse(-1, err.Error(), nil)
						return
					}
					if a, b := oneSqlRes.([]map[string]string); b {
						result = append(result, a...)
					}
				} else {
					switch {
					case "insert" == sqlInstance.Type:
						_, err := doInsert(*session, sqlInstance, jsonData, sqlApi.Params)
						if err != nil {
							framework.ProcessError(session.Rollback())
							context.ApiResponse(-1, err.Error(), nil)
							return
						}
						break
					case "select" == sqlInstance.Type:
						oneSqlRes, err := doSelect(*session, sqlInstance, jsonData, sqlApi.Params)
						if err != nil {
							framework.ProcessError(session.Rollback())
							context.ApiResponse(-1, err.Error(), nil)
							return
						}
						result = append(result, oneSqlRes...)
						break
					case "update" == sqlInstance.Type:
						_, err := doUpdate(*session, sqlInstance, jsonData)
						if err != nil {
							framework.ProcessError(session.Rollback())
							context.ApiResponse(-1, err.Error(), nil)
							return
						}
						break
					case "delete" == sqlInstance.Type:
						err := doDelete(*session, sqlInstance, jsonData)
						if err != nil {
							framework.ProcessError(session.Rollback())
							context.ApiResponse(-1, err.Error(), nil)
							return
						}
						break
					}
				}
			}
			result = append(result, sqlApi.Params)

			if sqlApi.Transaction {
				framework.ProcessError(session.Commit())
			}
			context.ApiResponse(0, "", result)
			return

		})
}
