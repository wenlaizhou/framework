package database

import (
	"strings"
	"fmt"
	"github.com/go-xorm/xorm"
	"github.com/wenlaizhou/framework/framework"
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

func exec(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{}, confParams map[string]string) {
	variable := make([]interface{}, 0)
	sql := sqlConf.SqlOrigin
	for _, p := range sqlConf.RParams {
		rp := new(SqlParam)
		rp.Key = p.Key
		switch p.Type {
		case Post:
			if confValue, ok := confParams[p.Key]; ok {
				rp.Value = confValue
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
			if reqValue, ok := requestJson[p.Key]; ok {
				pa.Value = reqValue
			} else {
				pa.Value = nil
			}
			variable = append(variable, pa.Value)
		}
	}
	if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
		session.QueryString(append([]interface{}{sql}, variable...))

	} else {
		session.Exec(sql, variable...)
	}
}

func doInsert(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{}, confParams map[string]string) {
	var values []interface{}
	var id = ""
	columnsStr := ""
	valuesStr := ""
	tableMeta := DbApiInstance.GetMeta(sqlConf.Table)
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k);
			column != nil && !column.IsAutoIncrement {
			if column.Name == "create_time" || column.Name == "update_time" {
				continue
			}
			if len(columnsStr) > 0 {
				columnsStr = fmt.Sprintf("%s, %s", columnsStr, column.Name)
			} else {
				columnsStr = column.Name
			}
			if len(valuesStr) > 0 {
				valuesStr = fmt.Sprintf("%s, ?", valuesStr)
			} else {
				valuesStr = "?"
			}
			values = append(values, v)
			continue
		}
	}
	//处理is_delete
	if isDelete := tableMeta.GetColumn("is_delete"); isDelete != nil {
		if len(columnsStr) > 0 {
			columnsStr = fmt.Sprintf("%s, %s", columnsStr, isDelete.Name)
		} else {
			columnsStr = isDelete.Name
		}
		if len(valuesStr) > 0 {
			valuesStr = fmt.Sprintf("%s, 0", valuesStr)
		} else {
			valuesStr = "0"
		}
	}
	primaryKey := tableMeta.GetColumn(tableMeta.PrimaryKeys[0]) // 限制单一主键
	//32位guid
	if primaryKey != nil && !primaryKey.IsAutoIncrement {
		columnsStr = fmt.Sprintf("%s, %s", columnsStr, primaryKey.Name)
		valuesStr = fmt.Sprintf("%s, ?", valuesStr)
		id = framework.Guid()
		values = append(values, id)
	}
	if createColumn := tableMeta.GetColumn("create_time"); createColumn != nil {
		columnsStr = fmt.Sprintf("%s, %s", columnsStr, createColumn.Name)
		valuesStr = fmt.Sprintf("%s, %s", valuesStr, "now()")
	}
	if updateColumn := tableMeta.GetColumn("update_time"); updateColumn != nil {
		columnsStr = fmt.Sprintf("%s, %s", columnsStr, updateColumn.Name)
		valuesStr = fmt.Sprintf("%s, %s", valuesStr, "now()")
	}
	sql := fmt.Sprintf("insert into %s (%s) values (%s);", tableMeta.Name, columnsStr, valuesStr)
	res, err := DbApiInstance.GetEngine().Exec(sql, values...)
	if !framework.ProcessError(err) {
		//记录日志
		logSql(logger, context, sql, values)
		//查询并写入es
		lastId, err := res.LastInsertId()
		framework.ProcessError(err)
		if len(id) > 0 {
			context.ApiResponse(0, "success", id)
		} else {
			context.ApiResponse(0, "success", lastId)
		}
	} else {
		context.ApiResponse(-1, err.Error(), nil)
	}
}

func doDelete(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{}, confParams map[string]string) {
	primaryValue, ok := requestJson["id"]
	if !ok || primaryValue == nil {
		context.ApiResponse(-1, "删除数据必须指定id值", nil)
		return
	}
	if len(tableMeta.PrimaryKeys) <= 0 {
		context.ApiResponse(-1, "表不存在主键, 无法删除数据", nil)
		return
	}
	tableMeta := DbApiInstance.GetMeta(sqlConf.Table)
	primaryKey := tableMeta.PrimaryKeys[0]
	sql := fmt.Sprintf("delete from %s where %s = ?;", tableMeta.Name, primaryKey)
	res, err := DbApiInstance.GetEngine().Exec(sql, primaryValue)
	if !framework.ProcessError(err) {
		logSql(logger, context, sql, []interface{}{primaryValue})
		rowsAffected, err := res.RowsAffected()
		if !framework.ProcessError(err) {
			context.ApiResponse(0, "success", rowsAffected)
			return
		} else {
			context.ApiResponse(-1, err.Error(), nil)
		}
	} else {
		context.ApiResponse(-1, err.Error(), nil)
		return
	}
}

func doUpdate(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{}, confParams map[string]string) {
	tableMeta := DbApiInstance.GetMeta(sqlConf.Table)
	primaryValue, ok := requestJson["id"]
	if !ok || primaryValue == nil {
		context.ApiResponse(-1, "删除数据必须指定id值", nil)
		return
	}
	primaryKey := tableMeta.PrimaryKeys[0]
	if len(tableMeta.PrimaryKeys) <= 0 {
		context.ApiResponse(-1, "表不存在主键, 无法更新", nil)
		return
	}
	var values []interface{}
	columnsStr := ""
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k);
			column != nil && !column.IsAutoIncrement {
			if column.Name == "create_time" || column.Name == "update_time" {
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
	res, err := DbApiInstance.GetEngine().Exec(sql, values...)
	if !framework.ProcessError(err) {
		logSql(logger, context, sql, values)
		rowsAffected, err := res.RowsAffected()
		if !framework.ProcessError(err) && rowsAffected == 1 {
			context.ApiResponse(0, "success", rowsAffected)
			return
		}
	}
	if err != nil {
		context.ApiResponse(-1, err.Error(), nil)
	}
	context.ApiResponse(-1, "", nil)
}

func doSelect(session xorm.Session, sqlConf SqlConf, requestJson map[string]interface{}, confParams map[string]string) {
	tableMeta := DbApiInstance.GetMeta(sqlConf.Table)
	var values []interface{}
	columnsStr := ""
	for k, v := range requestJson {
		if column := tableMeta.GetColumn(k); column != nil && !column.IsAutoIncrement {
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
	}

	sql := ""
	if len(columnsStr) <= 0 {
		sql = fmt.Sprintf("select * from %s;", tableMeta.Name)
	} else {
		sql = fmt.Sprintf("select * from %s where %s;", tableMeta.Name, columnsStr)
	}
	res, err := DbApiInstance.GetEngine().QueryString(append([]interface{}{sql}, values...)...)
	if !framework.ProcessError(err) {
		logSql(logger, context, sql, values)
	}
	if err != nil {
		context.ApiResponse(-1, err.Error(), nil)
		return
	}
	context.ApiResponse(0, "", res)
	return
}
