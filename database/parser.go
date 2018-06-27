package database

import (
	"strings"
	"fmt"
	"github.com/go-xorm/xorm"
	"github.com/wenlaizhou/framework/framework"
	"errors"
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
		return session.Exec(sql, variable...)
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

func doInsert(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{},
	confParams map[string]string) (interface{}, error) {

	var values []interface{}
	if confParams == nil {
		confParams = make(map[string]string)
	}
	var id = ""
	columnsStr := ""
	valuesStr := ""
	tableMeta := dbApiInstance.GetMeta(sqlConf.Table)
	for k, v := range confParams {
		requestJson[k] = v
	}
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k);
			column != nil && !column.IsAutoIncrement {
			if column.Name == "create_time" || column.Name == "update_time" {
				continue
			}
			columnsStr = appendColumnStr(columnsStr, column.Name)
			valuesStr = appendValueStr(valuesStr)
			if confValue, ok := confParams[k]; ok {
				if postReg.MatchString(confValue) {
					confMatch := postReg.FindAllStringSubmatch(confValue, -1)
					v = confParams[confMatch[0][1]]
				} else {
					v = confValue
				}
			}
			values = append(values, v)
			continue
		}
	}
	//处理is_delete
	if isDelete := tableMeta.GetColumn("is_delete"); isDelete != nil {
		columnsStr = appendColumnStr(columnsStr, isDelete.Name)
		if len(valuesStr) > 0 {
			valuesStr = fmt.Sprintf("%s, 0", valuesStr)
		} else {
			valuesStr = "0"
		}
	}

	if len(tableMeta.PrimaryKeys) > 0 {
		primaryKey := tableMeta.GetColumn(tableMeta.PrimaryKeys[0]) // 限制单一主键
		//32位guid
		if primaryKey != nil && !primaryKey.IsAutoIncrement {
			id = framework.Guid()
			columnsStr = appendColumnStr(columnsStr, primaryKey.Name)
			valuesStr = appendValueStr(valuesStr)
			if confValue, ok := confParams[primaryKey.Name]; ok { //id处理器
				if postReg.MatchString(confValue) {
					confMatch := postReg.FindAllStringSubmatch(confValue, -1)
					id = confParams[confMatch[0][1]]
				} else {
					id = confValue
				}
			}
			values = append(values, id)
		}
	}

	if createColumn := tableMeta.GetColumn("create_time"); createColumn != nil {
		columnsStr = appendColumnStr(columnsStr, createColumn.Name)
		if len(valuesStr) > 0 {
			valuesStr = fmt.Sprintf("%s, %s", valuesStr, "now()")
		} else {
			valuesStr = "now()"
		}
	}
	if updateColumn := tableMeta.GetColumn("update_time"); updateColumn != nil {
		columnsStr = appendColumnStr(columnsStr, updateColumn.Name)
		if len(valuesStr) > 0 {
			valuesStr = fmt.Sprintf("%s, %s", valuesStr, "now()")
		} else {
			valuesStr = "now()"
		}
	}
	sql := fmt.Sprintf("insert into %s (%s) values (%s);", tableMeta.Name, columnsStr, valuesStr)

	res, err := session.Exec(sql, values...)
	if framework.ProcessError(err) {
		return nil, err
	}
	if lid, err := res.LastInsertId(); err == nil {
		if len(id) > 0 {
			return id, nil
		}
		return lid, nil
	}
	return id, nil
}

func doDelete(session xorm.Session, sqlConf SqlConf,
	requestJson map[string]interface{}) (error) {

	tableMeta := dbApiInstance.GetMeta(sqlConf.Table)
	if len(tableMeta.PrimaryKeys) <= 0 {
		return errors.New("该表没有主键")
	}
	primaryValue, ok := requestJson[tableMeta.PrimaryKeys[0]]
	if !ok || primaryValue == nil {
		return errors.New(fmt.Sprintf("参数错误, 没有主键 %s", tableMeta.PrimaryKeys[0]))
	}

	primaryKey := tableMeta.PrimaryKeys[0]
	sql := fmt.Sprintf("delete from %s where %s = ?;", tableMeta.Name, primaryKey)
	_, err := session.Exec(sql, primaryValue)
	if !framework.ProcessError(err) {
		return err
	} else {
		return nil
	}
}

func doUpdate(session xorm.Session, sqlConf SqlConf,
	requestJson map[string]interface{}) (int64, error) {

	tableMeta := dbApiInstance.GetMeta(sqlConf.Table)
	if len(tableMeta.PrimaryKeys) <= 0 {
		return -1, errors.New("当前操作只支持有主键的表")
	}
	if len(requestJson) <= 1 {
		return -1, errors.New("参数错误, 数量过少")
	}
	primaryKey := tableMeta.PrimaryKeys[0]
	primaryValue, ok := requestJson[primaryKey]
	if !ok || primaryValue == nil {
		return -1, errors.New(fmt.Sprintf("参数错误, 没有主键 %s", tableMeta.PrimaryKeys[0]))
	}
	var values []interface{}
	columnsStr := ""
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k);
			column != nil && !column.IsAutoIncrement {
			if column.Name == "create_time" || column.Name == "update_time" {
				continue
			}
			if column.Name == primaryKey {
				continue
			}
			if len(columnsStr) > 0 {
				columnsStr = fmt.Sprintf("%s, %s = ?", columnsStr, column.Name)
			} else {
				columnsStr = fmt.Sprintf("%s = ?", column.Name)
			}
			values = append(values, v)
			continue
		}
	}
	if updateColumn := tableMeta.GetColumn("update_time"); updateColumn != nil {
		columnsStr = fmt.Sprintf("%s, %s=now()", columnsStr, updateColumn.Name)
	}
	sql := fmt.Sprintf("update %s set %s where %s = ?;", tableMeta.Name,
		columnsStr, primaryKey)
	values = append(values, primaryValue)
	res, err := session.Exec(sql, values...)
	if !framework.ProcessError(err) {
		return -1, err
	} else {
		return res.RowsAffected()
	}
}

func doSelect(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{},
	confParams map[string]string) ([]map[string]string, error) {

	tableMeta := dbApiInstance.GetMeta(sqlConf.Table)
	if len(requestJson) <= 0 && len(confParams) <= 0 {
		return session.QueryString(fmt.Sprintf("select * from %s;", tableMeta.Name), )
	}

	var values []interface{}
	columnsStr := ""
	for k, v := range confParams { //将配置写入到请求参数中
		if postReg.MatchString(v) {
			confMatch := postReg.FindAllStringSubmatch(v, -1)
			requestJson[k] = confParams[confMatch[0][1]]
		} else {
			requestJson[k] = v
		}
	}

	orderBySql := ""
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k);
			column != nil && !column.IsAutoIncrement {

			if len(columnsStr) > 0 {
				if realValues, ok := v.([]interface{}); ok && len(realValues) > 0 {
					rangeStr := ""
					for range realValues {
						rangeStr = fmt.Sprintf("%s, ?", rangeStr)
					}
					columnsStr = fmt.Sprintf("%s and %s in (%s)", columnsStr, column.Name, rangeStr[1:])
					values = append(values, realValues...)

				} else {
					if v != nil {
						columnsStr = fmt.Sprintf("%s and %s = ?", columnsStr, column.Name)
						values = append(values, v)
					}
				}

			} else {
				if realValues, ok := v.([]interface{}); ok && len(realValues) > 0 {
					rangeStr := ""
					for range realValues {
						rangeStr = fmt.Sprintf("%s, ?", rangeStr)
					}
					columnsStr = fmt.Sprintf("%s in (%s)", column.Name, rangeStr[1:])
					values = append(values, realValues...)

				} else {
					if v != nil {
						columnsStr = fmt.Sprintf("%s = ?", column.Name)
						values = append(values, v)
					}
				}

			}

			continue
		}
		if k == "order" { //order by 处理
			if v == nil {
				continue
			}
			switch v.(type) {
			case string:
				orderBySql = fmt.Sprintf("%s %s %s desc", orderBySql, "order by", v.(string))
				break
			case map[string]interface{}:
				/**
				order : {
					"key" : "asd",
					"desc" : true | false,
					"asc" : true | false
				}
				 */
				orderBy := v.(map[string]interface{})
				order, ok := orderBy["key"]
				if !ok {
					continue
				}
				descStr := "desc"
				desc, ok := orderBy["desc"].(bool)
				if ok && !desc {
					descStr = "asc"
				}
				asc, ok := orderBy["asc"].(bool)
				if ok && asc {
					descStr = "asc"
				}
				orderBySql = fmt.Sprintf("%s %s %v %s", orderBySql, "order by", order, descStr)
				break
			}
		}

	}

	// limit 处理
	limitSql := ""
	if start, ok := requestJson["start"]; ok {
		limitSql = fmt.Sprintf("limit %s", start)
		if size, ok := requestJson["size"]; ok {
			limitSql = fmt.Sprintf("%s, %s", limitSql, size)
		}
	}

	sql := ""
	if len(columnsStr) <= 0 {
		sql = fmt.Sprintf("select * from %s", tableMeta.Name)
	} else {
		sql = fmt.Sprintf("select * from %s where %s", tableMeta.Name, columnsStr)
	}
	sql = fmt.Sprintf("%s %s %s;", sql, orderBySql, limitSql)

	res, err := session.QueryString(append([]interface{}{sql}, values...)...)
	if !framework.ProcessError(err) {
		return res, err
	}
	return res, nil
}
