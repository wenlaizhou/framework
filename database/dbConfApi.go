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
	Path        string
	Transaction bool
	Sqls        []SqlApiSql
}

type SqlApiSql struct {
	SqlOrigin string
	RParams   []SqlParam
	Params    []SqlParam
	Id        string
}

type SqlParam struct {
	Type int
	Name string
	Id   string
}

const (
	Post   = 0 //${}
	Result = 1 //@{} result结果只能具有id类型
	//Replace = 2 //#{}
	//guid : {{guid}}
)

//四种类型参数
//1: post sql参数
//2: result sql参数
//3: post replace参数
//4: result replace参数
//5: guid {{guid}}

var postReg = regexp.MustCompile("\\$\\{(.*?)\\}")
var resultReplaceReg = "#\\{%s\\.(.*?)\\}"
var resultReg = "$\\{%s\\.(.*?)\\}"
var replaceReg = regexp.MustCompile("#\\{(.*?)\\}")

func InitSqlConfApi(filePath string) {
	apiConf := framework.LoadXml(filePath)
	apiElements := apiConf.FindElements("//sqlApi")
	sqlIds := make([]string, 0)
	for _, apiEle := range apiElements {
		sqlApi := new(SqlApi)
		sqlApi.Transaction = apiEle.SelectAttrValue("transaction", "") == "true"
		sqlApi.Path = apiEle.SelectAttrValue("path", "")
		sqlApi.Sqls = make([]SqlApiSql, 0)
		for i, sqlEle := range apiEle.FindElements(".//sql") {
			oneSql := new(SqlApiSql)
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

func initSqlApi(sqlApi SqlApi) { //todo sql id 相关配置需要进行细化代码
	if len(sqlApi.Path) <= 0 {
		log.Printf("sqlApi注册失败 : %#v 没有服务路径", sqlApi)
		return
	}
	log.Printf("注册sql api服务: %#v", sqlApi)
	framework.RegisterHandler(sqlApi.Path,
		func(context framework.Context) {
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

			result := make(map[string]interface{})

			for _, sqlInstance := range sqlApi.Sqls {
				realSql := sqlInstance.SqlOrigin

				//1: replace-process
				for _, rp := range sqlInstance.RParams {

					// post-replace
					if rp.Type == Post {
						v, ok := jsonData[rp.Name]
						if !ok {
							context.ApiResponse(-1,
								fmt.Sprintf("参数错误, 未包含 %s", rp.Name),
								nil)
							goto writeError
						}
						realSql = strings.Replace(realSql,
							fmt.Sprintf("#{%v}", rp.Name), v.(string), -1)
						continue

					} else if rp.Type == Result {

						//result-replace

						done := false

						v, ok := result[rp.Id]
						if !ok {
							context.ApiResponse(-1, //todo 整体错误处理
								fmt.Sprintf("参数错误, 未包含 %s", rp.Id),
								nil)
							goto writeError
						}
						sqlRes, ok := v.(sql.Result)
						if ok {
							id, _ := sqlRes.LastInsertId()
							realSql = strings.Replace(realSql,
								fmt.Sprintf("#{%v.%v}", rp.Id, rp.Name),
								strconv.FormatInt(id, 10), -1)
							done = true
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
								goto writeError
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
						goto writeError
					}
				}

				realSql = framework.ReplaceStr(realSql, "{{guid}}", framework.Guid)

				upperSql := strings.ToUpper(realSql)

				switch {

				case strings.HasPrefix(upperSql, "SELECT"):
					var args []interface{}
					for _, variable := range sqlInstance.Params {
						variable := variable
						if variable.Type == Post {
							param, ok := jsonData[variable.Name]
							if !ok {
								context.ApiResponse(-1, "未包含参数: "+variable.Name, nil)
								goto writeError
							}
							args = append(args, param)
						} else if variable.Type == Result {

							v, ok := result[variable.Id]
							if !ok {
								context.ApiResponse(-1, //todo 整体错误处理
									fmt.Sprintf("参数错误, 未包含 %s", variable.Id),
									nil)
								goto writeError
							}
							sqlRes, ok := v.(sql.Result)
							if ok {
								id, _ := sqlRes.LastInsertId()
								realSql = strings.Replace(realSql,
									fmt.Sprintf("#{%v.%v}", variable.Id, variable.Name),
									strconv.FormatInt(id, 10), -1)

							} else {
								sqlResMap, ok := v.([]map[string]string)
								if !ok {
									context.ApiResponse(-1, //todo 整体错误处理
										fmt.Sprintf("参数错误, 未包含id : %s", rp.Id),
										nil)
									goto writeError
								}
								if len(sqlResMap) <= 0 {
									context.ApiResponse(-1, //todo 整体错误处理
										fmt.Sprintf("参数错误, 没有查询结果 %s.%s", rp.Id, rp.Name),
										nil)
									goto writeError
								}
								vStr, ok := sqlResMap[0][rp.Name]
								if !ok {
									context.ApiResponse(-1, //todo 整体错误处理
										fmt.Sprintf("参数错误, 未包含列 : %s.%s", rp.Id, rp.Name),
										nil)
									goto writeError
								}
								realSql = strings.Replace(realSql,
									fmt.Sprintf("#{%v.%v}", rp.Id, rp.Name),
									vStr, -1)
							}

							param, ok := result[variable.Name]
							if !ok {
								context.ApiResponse(-1, "配置信息错误参数: "+variable.Name, nil)
								return
							}
							sqlRes, ok := param.(sql.Result)
							if !ok {
								context.ApiResponse(-1, "配置信息错误参数: "+variable.Name, nil)
								return
							} else {

								context.ApiResponse(-1, "该部分功能暂未实现", sqlRes)
							}
							//args = append(args, sqlRes.LastInsertId())
						}

					}
					res, err := session.QueryString(append([]interface{}{realSql}, args...)...)
					if !framework.ProcessError(err) {
						result[sqlInstance.Id] = res
					} else {
						if sqlApi.Transaction {
							framework.ProcessError(session.Rollback())
						}
						goto writeError
					}
					break

				case strings.HasPrefix(upperSql, "INSERT") ||
					strings.HasPrefix(upperSql, "DELETE") ||
					strings.HasPrefix(upperSql, "UPDATE"):
					var args []interface{}
					for _, variable := range sqlInstance.Params {
						variable := variable
						if variable.Type == Post {
							param, ok := jsonData[variable.Name]
							if !ok {
								param = nil
							}
							args = append(args, param)
						} else if variable.Type == Result {
							param, ok := result[variable.Name]
							if !ok {
								param = nil
							}
							args = append(args, param)
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
						goto writeError
					}
					break
				}
			}
			if sqlApi.Transaction {
				framework.ProcessError(session.Commit())
			} else {
				context.ApiResponse(0, "", result)
			}
			return

		writeError: //直接返回
			return //context.ApiResponse(-1, "sql执行错误", result)
		})
}
