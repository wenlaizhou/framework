package database

import (
	"github.com/go-xorm/core"
	"os"
	"fmt"
	"log"
	"encoding/json"
	"github.com/wenlaizhou/framework/framework"
	"strings"
)

var Tables []*core.Table

var tableMetas map[string]core.Table

var Config framework.Config

var inited = false

// 初始化数据库连接	, 配置:
// {
//	enableDbApi
// 	db.host
// 	db.port
//	db.user
//	db.password
//	db.database
// }
func InitDbApi(conf framework.Config) {

	Config = conf

	enableDbApi, ok := Config["enableDbApi"]
	if !ok || !enableDbApi.(bool) {
		return
	}
	_, ok = conf["logPath"]
	if !ok {
		conf["logPath"] = "logs"
	}
	if !framework.Exists(conf["logPath"].(string)) {
		framework.Mkdir(conf["logPath"].(string))
	}
	initDbApi()
	tablesMeta, err := dbApiInstance.GetEngine().DBMetas()
	if framework.ProcessError(err) {
		return
	}

	tableMetas = make(map[string]core.Table)

	for _, tableMeta := range tablesMeta {
		tableMeta := tableMeta
		Tables = append(Tables, tableMeta)
		tableMetas[tableMeta.Name] = *tableMeta
		registerTableCommonApi(*tableMeta)
	}
	registerTables()
	sqlLogPath := fmt.Sprintf("%s/sql.log", conf["logPath"])
	fs, err := os.OpenFile(sqlLogPath, os.O_CREATE|os.O_APPEND, os.ModePerm)
	if framework.ProcessError(err) {
		return
	}
	logger := log.New(fs, "", log.LstdFlags|log.Lshortfile)
	framework.RegisterHandler(fmt.Sprintf("/sql"),
		func(context framework.Context) { //安全
			jsonParam, err := context.GetJSON()
			if framework.ProcessError(err) {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}
			sql := jsonParam["sql"]
			if sql == nil {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}
			sqlStr, ok := sql.(string)
			if !ok {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}

			sqlStr = strings.TrimSpace(sqlStr)

			if len(sqlStr) <= 0 {
				context.ApiResponse(-1, "参数不包含sql", nil)
				return
			}

			if strings.Contains(strings.ToUpper(sqlStr), "DELETE") {
				context.ApiResponse(-1, "sql参数中不允许出现delete", nil)
				return
			}

			logSql(*logger, context, sqlStr, nil)
			res, err := dbApiInstance.GetEngine().QueryString(sqlStr)
			if !framework.ProcessError(err) {
				logger.Printf("%s\n, %s\n, %s\n",
					context.RemoteAddr(),
					string(context.Request.UserAgent()),
					sqlStr)
				context.ApiResponse(0, "", res)
			} else {
				context.ApiResponse(-1, err.Error(), res)
			}

		})
}

func registerTables() {
	framework.RegisterHandler("/tables", func(context framework.Context) {
		tablesBytes, _ := json.Marshal(Tables)
		tablesResult := string(tablesBytes)
		context.JSON(tablesResult)
		return
	})
}

func registerTableCommonApi(tableMeta core.Table) {
	logPath := fmt.Sprintf("%s/%s.log", Config["logPath"], tableMeta.Name)
	fs, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND, os.ModePerm)
	if framework.ProcessError(err) {
		return
	}
	logger := log.New(fs, "", log.LstdFlags|log.Lshortfile)
	registerTableInsert(tableMeta, *logger)
	registerTableUpdate(tableMeta, *logger)
	registerTableSelect(tableMeta, *logger)
	registerTableDelete(tableMeta, *logger)
	registerTableSchema(tableMeta)
}

func registerTableInsert(tableMeta core.Table, logger log.Logger) {
	framework.RegisterHandler(fmt.Sprintf("%s/insert", tableMeta.Name),
		func(context framework.Context) {
			params, err := context.GetJSON()
			if framework.ProcessError(err) || len(params) <= 0 {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}
			var values []interface{}
			var id = ""
			columnsStr := ""
			valuesStr := ""
			for k, v := range params {
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
			res, err := dbApiInstance.GetEngine().Exec(sql, values...)
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

		})
}

func registerTableDelete(tableMeta core.Table, logger log.Logger) {
	framework.RegisterHandler(fmt.Sprintf("%s/delete", tableMeta.Name),
		func(context framework.Context) {
			params, err := context.GetJSON()
			if err != nil || len(params) <= 0 {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}
			primaryValue, ok := params["id"]
			if !ok || primaryValue == nil {
				context.ApiResponse(-1, "删除数据必须指定id值", nil)
				return
			}
			if len(tableMeta.PrimaryKeys) <= 0 {
				context.ApiResponse(-1, "表不存在主键, 无法删除数据", nil)
				return
			}
			primaryKey := tableMeta.PrimaryKeys[0]
			sql := fmt.Sprintf("delete from %s where %s = ?;", tableMeta.Name, primaryKey)
			res, err := dbApiInstance.GetEngine().Exec(sql, primaryValue)
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
		})
}

func registerTableUpdate(tableMeta core.Table, logger log.Logger) {
	framework.RegisterHandler(fmt.Sprintf("%s/update", tableMeta.Name),
		func(context framework.Context) {
			params, err := context.GetJSON()
			if err != nil || len(params) <= 0 {
				context.ApiResponse(-1, "参数错误", nil)
				return
			}
			primaryValue, ok := params["id"]
			if !ok || primaryValue == nil {
				context.ApiResponse(-1, "修改数据必须指定id值", nil)
				return
			}
			primaryKey := tableMeta.PrimaryKeys[0]
			if len(tableMeta.PrimaryKeys) <= 0 {
				context.ApiResponse(-1, "表不存在主键, 无法更新", nil)
				return
			}
			var values []interface{}
			columnsStr := ""
			for k, v := range params {
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
			res, err := dbApiInstance.GetEngine().Exec(sql, values...)
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
		})
}

func registerTableSelect(tableMeta core.Table, logger log.Logger) {
	framework.RegisterHandler(fmt.Sprintf("%s/select", tableMeta.Name),
		func(context framework.Context) {
			params, err := context.GetJSON()
			var values []interface{}
			columnsStr := ""
			for k, v := range params {
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
				sql = fmt.Sprintf("select * from %s", tableMeta.Name)
			} else {
				sql = fmt.Sprintf("select * from %s where %s", tableMeta.Name, columnsStr)
			}
			//分页
			if start, ok := params["start"]; ok {
				if size, ok := params["size"]; ok {
					sql = fmt.Sprintf("%s limit %v, %v;", sql, start, size)
				}
			}
			res, err := dbApiInstance.GetEngine().QueryString(append([]interface{}{sql}, values...)...)
			if !framework.ProcessError(err) {
				logSql(logger, context, sql, values)
			}
			if err != nil {
				context.ApiResponse(-1, err.Error(), nil)
				return
			}
			context.ApiResponse(0, "", res)
			return
		})
}

func registerTableSchema(tableMeta core.Table) {
	framework.RegisterHandler(fmt.Sprintf("%s/schema", tableMeta.Name),
		func(context framework.Context) {
			context.ApiResponse(0, "",
				tableMeta.Columns())
		})
}
