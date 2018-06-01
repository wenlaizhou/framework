package database

import (
	"github.com/wenlaizhou/framework/framework"
	"strings"
	"fmt"
	"regexp"
	"strconv"
	"log"
	"database/sql"
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
			oneSql.Id = sqlEle.SelectAttrValue("id", strconv.Itoa(i))
			sqlIds = append(sqlIds, oneSql.Id)
			sqlStr := strings.TrimSpace(sqlEle.Text())

			//result post variable
			resultVariables := make([]SqlParam, 0)
			for _, id := range sqlIds {
				resultVariableNames := regexp.MustCompile(fmt.Sprintf(resultReg, id)).
					FindAllStringSubmatch(sqlStr, -1)

				for resList := range resultVariableNames {
					variableNameQute := resultVariableNames[resList][0]
					variableName := resultVariableNames[resList][1]
					variable := new(SqlParam)
					variable.Name = variableName
					variable.Id = id
					variable.Type = Result
					resultVariables = append(resultVariables, *variable)
					sqlStr = strings.Replace(sqlStr, variableNameQute, "?", 1)
				}

			}

			postVariables := make([]SqlParam, 0)
			postVariableNames := postReg.FindAllStringSubmatch(sqlStr, -1)
			for resList := range postVariableNames {
				variableNameQute := postVariableNames[resList][0]
				variableName := postVariableNames[resList][1]
				variable := new(SqlParam)
				_, ok := sqlApi.Params[variableName]
				if ok {
					variable.Id = variableName
					variable.Name = sqlApi.Params[variableName]
					variable.Type = Param
					sqlStr = strings.Replace(sqlStr, variableNameQute, "?", 1)
					continue
				}
				variable.Name = variableName
				variable.Type = Post
				postVariables = append(postVariables, *variable)
				sqlStr = strings.Replace(sqlStr, variableNameQute, "?", 1)
			}

			oneSql.Params = append(postVariables, resultVariables...)

			resultReplaceVariables := make([]SqlParam, 0)
			for _, id := range sqlIds {
				resultVariableNames := regexp.MustCompile(fmt.Sprintf(resultReplaceReg, id)).
					FindAllStringSubmatch(sqlStr, -1)

				for resList := range resultVariableNames {
					//variableNameQute := resultVariableNames[resList][0]
					variableName := resultVariableNames[resList][1]
					variable := new(SqlParam)
					variable.Name = variableName
					variable.Id = id
					variable.Type = Result
					resultReplaceVariables = append(resultReplaceVariables, *variable)
				}
			}

			replaceVariables := make([]SqlParam, 0)
			replaceVariableNames := replaceReg.FindAllStringSubmatch(sqlStr, -1)
			for resList := range replaceVariableNames {
				//variableNameQute := replaceVariableNames[resList][0]
				variableName := replaceVariableNames[resList][1]
				_, ok := sqlApi.Params[variableName]
				if ok {
					replaceVariables = append(replaceVariables, SqlParam{
						Name: sqlApi.Params[variableName],
						Id:   variableName,
						Type: Param,
					})
					continue
				}
				replaceVariables = append(replaceVariables, SqlParam{
					Name: variableName,
					Type: Post,
				})
			}
			oneSql.SqlOrigin = sqlStr
			oneSql.RParams = append(replaceVariables, resultReplaceVariables...)
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
			jsonData, err := context.GetJSON()
			if framework.ProcessError(err) {
				context.ApiResponse(-1, "参数错误, 非法json数据", nil)
				return
			}
			session := DbApiInstance.GetEngine().NewSession()
			defer session.Close()
			if sqlApi.Transaction {
				session.Begin()
			}

			result := make([]map[string]string, 0)

			for _, sqlInstance := range sqlApi.Sqls {
				realSql := sqlInstance.SqlOrigin

				//1: replace-process
				for _, rp := range sqlInstance.RParams {

					// post-replace
					switch {
					case rp.Type == Post:
						v, ok := jsonData[rp.Name]
						if !ok {
							context.ApiResponse(-1,
								fmt.Sprintf("参数错误, 未包含 %s", rp.Name),
								nil)
							return
						}
						realSql = strings.Replace(realSql,
							fmt.Sprintf("#{%v}", rp.Name), v.(string), -1)
						continue

					case rp.Type == Result:

						//result-replace

						v, ok := result[rp.Id]
						if !ok {
							context.ApiResponse(-1, //todo 整体错误处理
								fmt.Sprintf("参数错误, 未包含 %s", rp.Id),
								nil)
							return
						}
						sqlRes, ok := v.(sql.Result)
						if ok {
							id, _ := sqlRes.LastInsertId()
							realSql = strings.Replace(realSql,
								fmt.Sprintf("#{%v.%v}", rp.Id, rp.Name),
								strconv.FormatInt(id, 10), -1)
							continue

						}
						sqlResStr, ok := v.(string)
						if ok {
							realSql = strings.Replace(realSql,
								fmt.Sprintf("#{%v.%v}", rp.Id, rp.Name),
								sqlResStr, -1)
							continue
						}

						sqlResMap, ok := v.([]map[string]string)
						if ok {
							if len(sqlResMap) <= 0 {
								context.ApiResponse(-1,
									fmt.Sprintf("参数错误, 没有查询结果 %s.%s", rp.Id, rp.Name),
									nil)
								return
							}
							vStr, ok := sqlResMap[0][rp.Name]
							if ok {
								realSql = strings.Replace(realSql,
									fmt.Sprintf("#{%v.%v}", rp.Id, rp.Name),
									vStr, -1)
							}
						}

						context.ApiResponse(-1, //todo 整体错误处理
							fmt.Sprintf("参数错误, 未包含id : %s.%s", rp.Id, rp.Name),
							nil)
						return
					case rp.Type == Param:
						v, ok := sqlApi.Params[rp.Id]
						if !ok {
							context.ApiResponse(-1, //todo 整体错误处理
								fmt.Sprintf("参数错误, 未包含配置参数id : %s", rp.Id),
								nil)
							return
						}
						if v == "{{guid}}" {
							sqlApi.Params[rp.Id] = framework.Guid()
						}
						realSql = strings.Replace(realSql,
							fmt.Sprintf("#{%v}", rp.Id), sqlApi.Params[rp.Id], -1)
					}
				}

				realSql = framework.ReplaceStr(realSql, "{{guid}}", framework.Guid)

				upperSql := strings.ToUpper(realSql)

				switch {

				case strings.HasPrefix(upperSql, "SELECT"):
					var args []interface{}
					for _, variable := range sqlInstance.Params {
						variable := variable
						switch {
						case variable.Type == Post:
							param, ok := jsonData[variable.Name]
							if !ok {
								context.ApiResponse(-1, "未包含参数: "+variable.Name, nil)
								return
							}
							args = append(args, param)
						case variable.Type == Result:

							v, ok := result[variable.Id]
							if !ok {
								context.ApiResponse(-1, //todo 整体错误处理
									fmt.Sprintf("参数错误, 未包含 %s", variable.Id),
									nil)
								return
							}
							sqlRes, ok := v.(sql.Result)
							if ok {
								id, _ := sqlRes.LastInsertId()
								args = append(args, id)
								continue
							}

							sqlResMap, ok := v.([]map[string]string)
							if ok {
								if len(sqlResMap) <= 0 {
									args = append(args, "")
									continue
								}
								vStr, ok := sqlResMap[0][variable.Name]
								if ok {
									args = append(args, vStr)
									continue
								}
							}

							sqlResStr, ok := v.(string)
							if ok {
								args = append(args, sqlResStr)
								continue
							}

							context.ApiResponse(-1, "配置信息错误参数: "+variable.Name, nil)
							return
							//args = append(args, sqlRes.LastInsertId())

						case variable.Type == Param:
							v, ok := sqlApi.Params[variable.Id]
							if !ok {
								context.ApiResponse(-1, //todo 整体错误处理
									fmt.Sprintf("参数错误, 未包含配置参数id : %s", variable.Id),
									nil)
								return
							}
							if v == "{{guid}}" {
								sqlApi.Params[variable.Id] = framework.Guid()
							}
							args = append(args, sqlApi.Params[variable.Id])
						}

					}
					res, err := session.QueryString(append([]interface{}{realSql}, args...)...)
					if !framework.ProcessError(err) {
						result = append(result, res...)
					} else {
						if sqlApi.Transaction {
							framework.ProcessError(session.Rollback())
						}
						context.ApiResponse(-1, "sql执行错误 : "+realSql, args)
						return
					}
					break

				case strings.HasPrefix(upperSql, "INSERT") ||
					strings.HasPrefix(upperSql, "DELETE") ||
					strings.HasPrefix(upperSql, "UPDATE"):
					var args []interface{}
					for _, variable := range sqlInstance.Params {
						variable := variable

						switch {
						case variable.Type == Post:
							param, ok := jsonData[variable.Name]
							if !ok {
								context.ApiResponse(-1, "未包含参数: "+variable.Name, nil)
								return
							}
							args = append(args, param)
						case variable.Type == Result:

							v, ok := result[variable.Id]
							if !ok {
								context.ApiResponse(-1, //todo 整体错误处理
									fmt.Sprintf("参数错误, 未包含 %s", variable.Id),
									nil)
								return
							}
							sqlRes, ok := v.(sql.Result)
							if ok {
								id, _ := sqlRes.LastInsertId()
								args = append(args, id)
								continue
							}

							sqlResMap, ok := v.([]map[string]string)
							if ok {
								if len(sqlResMap) <= 0 {
									args = append(args, "")
									continue
								}
								vStr, ok := sqlResMap[0][variable.Name]
								if ok {
									args = append(args, vStr)
									continue
								}
							}

							sqlResStr, ok := v.(string)
							if ok {
								args = append(args, sqlResStr)
								continue
							}

							context.ApiResponse(-1, "配置信息错误参数: "+variable.Name, nil)
							return
							//args = append(args, sqlRes.LastInsertId())
						case variable.Type == Param:
							v, ok := sqlApi.Params[variable.Id]
							if !ok {
								context.ApiResponse(-1, //todo 整体错误处理
									fmt.Sprintf("参数错误, 未包含配置参数id : %s", variable.Id),
									nil)
								return
							}
							if v == "{{guid}}" {
								sqlApi.Params[variable.Id] = framework.Guid()
							}
							args = append(args, sqlApi.Params[variable.Id])
						}

					}
					res, err := session.Exec(realSql, args...)
					if !framework.ProcessError(err) {
						if res != nil {
							framework.ProcessError(err)
							result[sqlInstance.Id] = res
						}
					} else {
						if sqlApi.Transaction {
							framework.ProcessError(session.Rollback())
						}
						context.ApiResponse(-1, "sql 执行失败: "+realSql, args)
						return
					}
					break
				}
			}
			if sqlApi.Transaction {
				framework.ProcessError(session.Commit())
			}
			context.ApiResponse(0, "", result)
			return

		})
}
