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

// 初始化数据库连接
// 调用该方法可重复更新配置, 重新创建连接
// 
// 配置:
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
			logger.Printf("获取insert调用: %v", params)
			id, err := doInsert(*GetEngine().NewSession(), SqlConf{
				Id:    tableMeta.Name,
				Table: tableMeta.Name,
			}, params, nil)
			if err != nil {
				context.ApiResponse(-1, err.Error(), nil)
				return
			}
			context.ApiResponse(0, "", id)
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
			logger.Printf("获取delete调用: %v", params)
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
			logger.Printf("获取update调用: %v", params)
			res, err := doUpdate(*GetEngine().NewSession(), SqlConf{
				Table: tableMeta.Name,
			}, params)
			if err != nil {
				context.ApiResponse(-1, err.Error(), nil)
				return
			} else {
				context.ApiResponse(0, "success", res)
				return
			}
		})
}

func registerTableSelect(tableMeta core.Table, logger log.Logger) {
	framework.RegisterHandler(fmt.Sprintf("%s/select", tableMeta.Name),
		func(context framework.Context) {
			params, err := context.GetJSON()
			if err != nil {
				params = nil
			}
			logger.Printf("获取select调用: %v", params)
			res, err := doSelect(*GetEngine().NewSession(), SqlConf{
				Table:  tableMeta.Name,
				HasSql: false,
			}, params, nil)
			if framework.ProcessError(err) {
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
