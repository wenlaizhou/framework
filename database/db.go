/**
 *                                         ,s555SB@@&amp;
 *                                      :9H####@@@@@Xi
 *                                     1@@@@@@@@@@@@@@8
 *                                   ,8@@@@@@@@@B@@@@@@8
 *                                  :B@@@@X3hi8Bs;B@@@@@Ah,
 *             ,8i                  r@@@B:     1S ,M@@@@@@#8;
 *            1AB35.i:               X@@8 .   SGhr ,A@@@@@@@@S
 *            1@h31MX8                18Hhh3i .i3r ,A@@@@@@@@@5
 *            ;@&amp;i,58r5                 rGSS:     :B@@@@@@@@@@A
 *             1#i  . 9i                 hX.  .: .5@@@@@@@@@@@1
 *              sG1,  ,G53s.              9#Xi;hS5 3B@@@@@@@B1
 *               .h8h.,A@@@MXSs,           #@H1:    3ssSSX@1
 *               s ,@@@@@@@@@@@@Xhi,       r#@@X1s9M8    .GA981
 *               ,. rS8H#@@@@@@@@@@#HG51;.  .h31i;9@r    .8@@@@BS;i;
 *                .19AXXXAB@@@@@@@@@@@@@@#MHXG893hrX#XGGXM@@@@@@@@@@MS
 *                s@@MM@@@hsX#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@&amp;,
 *              :GB@#3G@@Brs ,1GM@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@B,
 *            .hM@@@#@@#MX 51  r;iSGAM@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@8
 *          :3B@@@@@@@@@@@&amp;9@h :Gs   .;sSXH@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@:
 *      s&amp;HA#@@@@@@@@@@@@@@M89A;.8S.       ,r3@@@@@@@@@@@@@@@@@@@@@@@@@@@r
 *   ,13B@@@@@@@@@@@@@@@@@@@5 5B3 ;.         ;@@@@@@@@@@@@@@@@@@@@@@@@@@@i
 *  5#@@#&amp;@@@@@@@@@@@@@@@@@@9  .39:          ;@@@@@@@@@@@@@@@@@@@@@@@@@@@;
 *  9@@@X:MM@@@@@@@@@@@@@@@#;    ;31.         H@@@@@@@@@@@@@@@@@@@@@@@@@@:
 *   SH#@B9.rM@@@@@@@@@@@@@B       :.         3@@@@@@@@@@@@@@@@@@@@@@@@@@5
 *     ,:.   9@@@@@@@@@@@#HB5                 .M@@@@@@@@@@@@@@@@@@@@@@@@@B
 *           ,ssirhSM@&amp;1;i19911i,.             s@@@@@@@@@@@@@@@@@@@@@@@@@@S
 *              ,,,rHAri1h1rh&amp;@#353Sh:          8@@@@@@@@@@@@@@@@@@@@@@@@@#:
 *            .A3hH@#5S553&amp;@@#h   i:i9S          #@@@@@@@@@@@@@@@@@@@@@@@@@A.
 *
 */
package database

import _ "github.com/go-sql-driver/mysql"
import (
	"github.com/go-xorm/xorm"
	"fmt"
	"log"
	"reflect"
	"strings"
	"encoding/json"
	"regexp"
	"strconv"
	"github.com/go-xorm/core"
	"sync"
	"github.com/wenlaizhou/framework/framework"
)

type TableHandler struct {
	TableHolder core.Table
	ApiPath     string
	TableName   string
}

type DbApi struct {
	host       string
	port       int
	user       string
	password   string
	db         string
	datasource string
	orm        *xorm.Engine
	dataStruct map[string]reflect.Type
}

var dbApiInstance *DbApi

var dbApiInstanceLock = new(sync.Mutex)

func NewDbApi(host string,
	port int,
	user string,
	password string,
	db string) (*DbApi, error) {
	res := &DbApi{
		host:       host,
		port:       port,
		user:       user,
		password:   password,
		db:         db,
		dataStruct: make(map[string]reflect.Type),
	}
	res.datasource = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8",
		res.user, res.password, res.host, res.port, res.db)
	orm, err := xorm.NewEngine("mysql", res.datasource)
	//orm, err := xorm.NewEngine("sqlite3", "data.db")
	if err != nil {
		log.Println("数据库连接错误")
		log.Println(err.Error())
		return nil, err
	}
	orm.ShowSQL(true)
	res.orm = orm
	return res, nil
}

func initDbApi() {

	dbApiInstanceLock.Lock()
	defer dbApiInstanceLock.Unlock()

	if dbApiInstance != nil {
		return
	}
	var err error
	dbApiInstance, err = NewDbApi(
		Config["db.host"].(string),
		int(Config["db.port"].(float64)),
		Config["db.user"].(string),
		Config["db.password"].(string),
		Config["db.database"].(string))
	if framework.ProcessError(err) {
		return
	}
	dbApiInstance.GetEngine().ShowSQL(true)
}

func (this *DbApi) GetStruct() map[string]map[string]string {
	res := make(map[string]map[string]string)
	for table, st := range this.dataStruct {
		columnStruct := make(map[string]string)
		for i := 0; i < st.NumField(); i++ {
			columnFd := st.Field(i)
			columnStruct[columnFd.Tag.Get("json")] = columnFd.Type.String()
		}
		res[table] = columnStruct
	}
	return res
}

func (this *DbApi) GetMeta(tableName string) core.Table {
	return tableMetas[tableName]
}

func (this *DbApi) GetEngine() *xorm.Engine {
	return this.orm
}

// 获取
func GetMeta(tableName string) core.Table {
	return dbApiInstance.GetMeta(tableName)
}

// 获取数据库引擎
func GetEngine() *xorm.Engine {
	return dbApiInstance.GetEngine()
}

func (this *DbApi) RegisterDbApi(orm interface{}) {
	ormValue := reflect.ValueOf(orm)
	if ormValue.Kind() != reflect.Ptr {
		log.Println("orm 对象必须是指针")
		return
	}
	ormType := ormValue.Elem().Type()
	log.Println("开始注册 : ", orm.(xorm.TableName).TableName())
	log.Printf("%#v\n", orm)
	this.dataStruct[orm.(xorm.TableName).TableName()] = ormType
	primaryIndex := -1
	for i := 0; i < ormType.NumField(); i++ {
		tag := ormType.Field(i).Tag.Get("xorm")
		log.Println(tag)
		if strings.Contains(tag, "primary") {
			primaryIndex = i
			break
		}
	}
	log.Println(primaryIndex)
	isExist, err := this.orm.Exist(orm)
	framework.ProcessError(err)
	if !isExist {
		this.orm.CreateTables(orm)
	}

	framework.RegisterHandler(fmt.Sprintf("/%s/insert", orm.(xorm.TableName).TableName()),
		func(ctx framework.Context) {
			resValue := reflect.New(ormType) //INSERT INTO .. ON DUPLICATE KEY UPDATE
			json.Unmarshal([]byte(ctx.GetBody()), resValue.Interface())
			log.Printf("%#v", resValue.Interface())
			_, err := this.orm.Insert(resValue.Interface())
			if err != nil {
				log.Println(err.Error())
				ctx.ApiResponse(-1, "", nil)
				return
			}
			ctx.ApiResponse(0, "", nil)
		})

	framework.RegisterHandler(fmt.Sprintf("/%s/update", orm.(xorm.TableName).TableName()),
		func(ctx framework.Context) {
			resValue := reflect.New(ormType)
			json.Unmarshal([]byte(ctx.GetBody()), resValue.Interface())
			condition := make(map[string]int)
			condition["id"], _ = strconv.Atoi(ctx.Request.URL.Query().Get("id"))
			_, err := this.orm.Update(resValue.Interface(), condition)
			if err != nil {
				log.Println(err.Error())
				ctx.ApiResponse(-1, "", nil)
				return
			}
			ctx.ApiResponse(0, "", nil)
		})

	framework.RegisterHandler(fmt.Sprintf("/%s/delete", orm.(xorm.TableName).TableName()),
		func(ctx framework.Context) {
			id, _ := strconv.Atoi(ctx.Request.URL.Query().Get("id"))
			_, err := this.orm.Delete(map[string]interface{}{"id": id})
			if err != nil {
				log.Println(err.Error())
				ctx.ApiResponse(-1, "", nil)
				return
			}
			ctx.ApiResponse(0, "", nil)
		})

	framework.RegisterHandler(fmt.Sprintf("/%s/select", orm.(xorm.TableName).TableName()),
		func(ctx framework.Context) {
			resValue := reflect.New(ormType)
			json.Unmarshal([]byte(ctx.GetBody()), resValue.Interface())
			res := reflect.New(reflect.SliceOf(ormType)).Interface()
			err := this.orm.Find(res, resValue.Interface())
			if err != nil {
				log.Println(err.Error())
				ctx.ApiResponse(-1, "", nil)
				return
			}
			ctx.ApiResponse(0, "", &res)
		})
}

var reg, _ = regexp.Compile("\\$\\{(.*?)\\}")
var idReg, _ = regexp.Compile("(\\d+)\\.id")

//写入动作日志
func logSql(logger log.Logger, request framework.Context, sql string, values []interface{}) {
	logger.Printf("%s, %s\n, %s\n, %#v\n",
		request.RemoteAddr(),
		string(request.Request.UserAgent()), sql, values)
}

//sql 拆解
func explainSql(sql string, ids *[]string) (string, []string) {
	variableNames := reg.FindAllStringSubmatch(sql, -1)
	var variables []string

	for resList := range variableNames {
		resList := resList
		variableNameQute := variableNames[resList][0]
		variableName := variableNames[resList][1]
		switch {
		case variableName == "guid":
			id := framework.Guid()
			sql = strings.Replace(sql, variableNameQute,
				fmt.Sprintf("\"%s\"", id), 1)
			*ids = append(*ids, id)
			break
		case idReg.MatchString(variableName):
			posStr := idReg.FindAllStringSubmatch(variableName, -1)
			pos, err := strconv.ParseInt(posStr[0][1], 10, 0)
			framework.ProcessError(err)
			sql = strings.Replace(sql, variableNameQute,
				fmt.Sprintf("\"%s\"", (*ids)[pos]), 1)
			break
		default:
			sql = strings.Replace(sql, variableNameQute, "?", 1)
			variables = append(variables, variableName)
			break
		}

	}
	return sql, variables
}
