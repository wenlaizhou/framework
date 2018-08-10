package database

import (
	"strings"
	"fmt"
	"github.com/go-xorm/xorm"
)

// 提炼sql语句中的参数
//
// 1. 替换参数
//
// 2. 参数参数
//
// 3. 暂时不处理结果参数
func parseSql(sql string) (string, []SqlParam, []SqlParam) {

	postVariables := make([]SqlParam, 0)
	postVariableNames := postReg.FindAllStringSubmatch(sql, -1)
	for resList := range postVariableNames {
		variableNameQute := postVariableNames[resList][0]
		variableName := postVariableNames[resList][1]
		variable := new(SqlParam)
		variable.Key = variableName
		variable.Type = Post
		postVariables = append(postVariables, *variable)
		sql = strings.Replace(sql, variableNameQute, "?", 1)
	}

	replaceVariables := make([]SqlParam, 0)
	replaceVariableNames := replaceReg.FindAllStringSubmatch(sql, -1)
	for resList := range replaceVariableNames {
		//variableNameQute := replaceVariableNames[resList][0]
		variableName := replaceVariableNames[resList][1]
		replaceVariables = append(replaceVariables, SqlParam{
			Key:  variableName,
			Type: Post,
		})
	}

	return sql, replaceVariables, postVariables
}

func exec(session xorm.Session, sqlConf SqlConf,
	requestJson map[string]interface{}, confParams map[string]string) (interface{}, error) {

	variable := make([]interface{}, 0)
	sql := sqlConf.SqlOrigin
	for _, p := range sqlConf.RParams {
		rp := new(SqlParam)
		rp.Key = p.Key
		switch p.Type {
		case Post:
			if confValue, ok := confParams[p.Key]; ok {
				if postReg.MatchString(confValue) {
					confMatch := postReg.FindAllStringSubmatch(confValue, -1)
					rp.Value = confParams[confMatch[0][1]]
				} else {
					rp.Value = confValue
				}
			} else {
				if reqValue, ok := requestJson[p.Key]; ok {
					rp.Value = fmt.Sprintf("%v", reqValue)
				} else {
					rp.Value = ""
				}
			}
			sql = strings.Replace(sql, fmt.Sprintf("#{%s}", rp.Key),
				rp.Value.(string), -1)

			break
		}
	}
	for _, p := range sqlConf.Params {
		pa := new(SqlParam)
		pa.Key = p.Key
		switch p.Type {
		case Post:
			if confValue, ok := confParams[p.Key]; ok {
				if postReg.MatchString(confValue) {
					confMatch := postReg.FindAllStringSubmatch(confValue, -1)
					pa.Value = confParams[confMatch[0][1]]
				} else {
					pa.Value = confValue
				}
			} else {
				if reqValue, ok := requestJson[p.Key]; ok {
					pa.Value = reqValue
				} else {
					pa.Value = nil
				}
			}
			variable = append(variable, pa.Value)
		}
	}
	if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
		return session.QueryString(append([]interface{}{sql}, variable...)...)

	} else {
		return session.Exec(append([]interface{}{sql}, variable...)...)
	}
}

func appendColumnStr(columnsStr string, columnName string) string {
	if len(columnName) <= 0 {
		return columnsStr
	}
	if len(columnsStr) > 0 {
		return fmt.Sprintf("%s, %s", columnsStr, columnName)
	} else {
		return columnName
	}
}

func appendValueStr(valuesStr string) string {
	if len(valuesStr) > 0 {
		return fmt.Sprintf("%s, ?", valuesStr)
	} else {
		return "?"
	}
}
