package myContext

import (
	"encoding/json"
	"errors"
	"fmt"
	"gosab/system/core/app"
	"gosab/system/core/commonFunc"
	"gosab/system/core/log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type Context struct {
	App *app.App

	Request *http.Request
	Writer  http.ResponseWriter
	Header  http.Header

	Controller string
	Method     string

	Log *log.NLog //记录日志使用

	Post    url.Values //单纯的form-data请求数据 或者x-www-form-urlencoded请求数据
	GetPost url.Values //get参数与 form-data或者x-www-form-urlencoded合集

	CommonParam CommonParam //公共参数 验证签名的请求使用
	YewuParam   YewuParam   //业务参数 验证签名的请求使用
}

//初始化 上下文请求信息
func NewContext(r *http.Request, w http.ResponseWriter, app *app.App) (*Context, error) {
	uri := r.RequestURI

	defaultClass := "welcome"
	defaultMethod := "Index"
	cClass := ""
	cMethod := ""
	if uri == "/" {
		cClass = defaultClass
		cMethod = defaultMethod
	} else {
		splitUri := strings.Split(uri, "/")
		cClass = splitUri[1]
		if len(splitUri) >= 3 {
			if splitUri[2] == "" {
				cMethod = defaultMethod
			} else {
				cMethod = strings.ToUpper(splitUri[2][:1]) + splitUri[2][1:]
			}
		} else {
			cMethod = defaultMethod
		}
	}

	context := &Context{
		App:        app,
		Request:    r,
		Writer:     w,
		Controller: cClass,
		Method:     cMethod,
		Log:        app.Log,
	}

	//解析请求参数
	context.parseForm()

	ip := commonFunc.ClientIP(r)

	//记录访问日志
	context.Log.Info(ip + ":" + cClass + "/" + cMethod + "--" + r.Header.Get("User-Agent"))

	//验证ip是否可访问
	if !app.IpClass(cClass, ip) {
		_, _ = fmt.Fprintf(w, string("受限的ip访问: "+ip))
		return nil, errors.New("受限的ip访问: " + ip)
	}

	//签名检查 检查是否验证签名信息
	b, err := context.signCheck()
	if !b {
		return nil, err
	}

	return context, nil
}

//签名检查 并将需要签名的参数 拆分参数为公共参数和业务参数
func (this *Context) signCheck() (bool, error) {
	if !this.App.SIGNCHECK { //不需要检查
		return true, errors.New("不用检查")
	}

	//检查class 是否在 不需要检查的里面
	if this.App.NSIGN != "" {
		cNames := strings.Split(this.App.NSIGN, ",")
		for _, v := range cNames {
			if v == this.Controller {
				return true, errors.New("不用检查的制订")
			}
		}
	}

	//查看是否包含param参数 固定格式
	if this.Post["param"] == nil {
		return false, errors.New("post提交缺少param参数")
	}

	//解析参数 分成公共参数 和 业务参数 并判断签名
	err := this.exParam(this.Post["param"][0])
	if err != nil {
		return false, err
	}

	return true, nil
}

//定义业务参数
type YewuParam = map[string]interface{}

//公共参数
type CommonParam struct {
	AppKey     string    `json:"app_key"`
	Control    string    `json:"control"`
	Method     string    `json:"method"`
	FromSource int64     `json:"from_source"`
	Ip         string    `json:"ip"`
	Sign       string    `json:"sign"`
	Param      YewuParam `json:"param"` //业务参数部分
}

//解析签名参数 分成公共参数 和 业务参数 并判断签名
func (this *Context) exParam(exParam string) error {
	data := CommonParam{}
	err := json.Unmarshal([]byte(exParam), &data)
	if err != nil {
		return err
	}

	if data.AppKey == "" {
		return errors.New("app_key不能为空")
	}

	if data.Control == "" {
		return errors.New("control不能为空")
	}

	if data.Method == "" {
		return errors.New("method不能为空")
	}

	if data.Ip == "" {
		return errors.New("IP不能为空")
	}

	if data.Sign == "" {
		return errors.New("签名不能为空")
	}

	this.CommonParam = data
	this.YewuParam = data.Param

	//TODO 生成签名比较对错
	/*if data.Sign != this.sign() {
		return errors.New("签名不匹配")
	}*/

	//TODO 可以用来记录日志

	return nil
}

//升级请求为websocket
func (this *Context) WebSocket() (*websocket.Conn, error) {
	upgrade := websocket.Upgrader{}
	upgrade.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	conn, err := upgrade.Upgrade(this.Writer, this.Request, nil)
	return conn, err
}

//TODO 生成签名参数
func (this *Context) sign() string {
	return ""
}

func (this *Context) GetParam(param string) string {
	if this.GetPost[param] != nil {
		return this.GetPost[param][0]
	} else {
		return ""
	}
}

/**
param 要检查的字段名
dec 字段描述
request 是否必填
min 最小值
max 最大允许值
*/
func (this *Context) CheckParamInt(param string, dec string, request bool, min int64, max int64) (int64, error) {
	var tmp string
	if request { //判断必填
		if this.GetPost[param] == nil {
			return 0, errors.New(dec + "不能为空")
		}
		tmp = strings.TrimSpace(this.GetPost[param][0])
		if tmp == "" {
			return 0, errors.New(dec + "不能为空格等空字符")
		}
	} else {
		if this.GetPost[param] == nil {
			tmp = ""
		} else {
			tmp = strings.TrimSpace(this.GetPost[param][0])
		}
	}

	paramInt, _ := strconv.ParseInt(tmp, 10, 64)

	if paramInt < min {
		return 0, errors.New(dec + "不能小于" + strconv.FormatInt(min, 10))
	}

	if max == 0 { //限定下 数据库int 普通情况 11位 够用即可
		max = 99999999999
	}

	if paramInt > max {
		return 0, errors.New(dec + "不能大于" + strconv.FormatInt(max, 10))
	}

	return paramInt, nil
}

/**
GET POST参数检测并转换为string
min 0不判断最小长度
max 0 强制max = 65535 判断最大长度
*/
func (this *Context) CheckParamString(param string, dec string, request bool, min int64, max int64) (string, error) {
	var tmp string
	if request {
		if this.GetPost[param] == nil {
			return "", errors.New(dec + "不能为空")
		}
		tmp = strings.TrimSpace(this.GetPost[param][0])
		if tmp == "" {
			return "", errors.New(dec + "不能为空格等空字符")
		}
	} else {
		if this.GetPost[param] == nil {
			tmp = ""
		} else {
			tmp = strings.TrimSpace(this.GetPost[param][0])
		}
	}

	//获得字符长度
	num := int64(utf8.RuneCountInString(tmp))

	//判断长度
	if min > 0 {
		if num < min {
			return "", errors.New(dec + "最少" + strconv.FormatInt(min, 10) + "个字")
		}
	}

	if max == 0 { //限定下 数据库存储 普通情况 最大65535
		max = 65535
	}

	if num > max {
		return "", errors.New(dec + "最多" + strconv.FormatInt(max, 10) + "个字")
	}

	return tmp, nil
}

func (this *Context) GetParamInt(param string) int64 {
	if this.GetPost[param] != nil {
		paramint, _ := strconv.ParseInt(this.GetPost[param][0], 10, 64)
		return paramint
	} else {
		return 0
	}
}

//按字段名 获取业务参数
func (this *Context) YewuParamInt(param string, dec string, request bool, min int64, max int64) (int64, error) {
	if this.App.ENV == "local" { //本地环境时切换处理函数
		return this.CheckParamInt(param, dec, request, min, max)
	}

	if request {
		if this.YewuParam[param] == nil {
			return 0, errors.New(dec + "不能为空")
		}
	}

	var paramInt int64
	if this.YewuParam[param] != nil {
		switch this.YewuParam[param].(type) {
		case int:
			paramInt = int64(this.YewuParam[param].(int))
		case int64:
			paramInt = this.YewuParam[param].(int64)
		case string:
			paramInt, _ = strconv.ParseInt(this.YewuParam[param].(string), 10, 64)
		case float64:
			paramInt = int64(this.YewuParam[param].(float64))
		default:
			paramInt = 0
		}
	} else {
		paramInt = 0
	}

	if paramInt < min {
		return 0, errors.New(dec + "不能小于" + strconv.FormatInt(min, 10))
	}

	if max == 0 {
		max = 9999999999
	}
	if paramInt > max {
		return 0, errors.New(dec + "不能大于" + strconv.FormatInt(max, 10))
	}

	return paramInt, nil
}

/**
业务参数检测并转换为string
*/
func (this *Context) YewuParamString(param string, dec string, request bool, min int64, max int64) (string, error) {
	if this.App.ENV == "local" { //本地环境时切换处理函数
		return this.CheckParamString(param, dec, request, min, max)
	}

	if request {
		if this.YewuParam[param] == nil {
			return "", errors.New(dec + "不能为空")
		}
	}

	var ps string
	if this.YewuParam[param] != nil {
		switch this.YewuParam[param].(type) {
		case int:
			ps = strconv.Itoa(this.YewuParam[param].(int))
		case int64:
			ps = strconv.FormatInt(this.YewuParam[param].(int64), 10)
		case string:
			ps = this.YewuParam[param].(string)
		case float64:
			ps = strconv.FormatFloat(this.YewuParam[param].(float64), 'f', -1, 64)
		default:
			ps = ""
		}
	} else {
		ps = ""
	}

	//获得字符长度
	num := int64(utf8.RuneCountInString(ps))

	//判断长度
	if min > 0 {
		if num < min {
			return "", errors.New(dec + "最少" + strconv.FormatInt(min, 10) + "个字")
		}
	}

	if max == 0 {
		max = 65535
	}

	if num > max {
		return "", errors.New(dec + "最多" + strconv.FormatInt(max, 10) + "个字")
	}

	return ps, nil
}

//解析请求参数
func (this *Context) parseForm() {
	_ = this.Request.ParseForm()                  //解析get参数
	_ = this.Request.ParseMultipartForm(32 << 20) //解析post参数

	this.GetPost = this.Request.Form
	this.Post = this.Request.PostForm
	//c.fileData = c.Request.MultipartForm.File
}

//json返回数据
type JsonErr struct {
	Msg  string      `json:"msg"`
	Ret  int         `json:"ret"`
	Data interface{} `json:"data"`
}

func (this *Context) JsonReturn(code int, data interface{}, format string, a ...interface{}) {
	msg := ""

	if strings.Contains(format, "%s") || strings.Contains(format, "%d") || strings.Contains(format, "%v") || strings.Contains(format, "%t") {
		msg = fmt.Sprintf(format, a)
	} else {
		msg = format
	}

	st := &JsonErr{
		Msg:  msg,
		Ret:  code,
		Data: data,
	}

	b, _ := json.Marshal(st)

	header := this.Writer.Header()
	header.Set("Content-Type", "application/json;charset=UTF-8")
	_, _ = fmt.Fprintf(this.Writer, string(b))
}

//txt内容直接返回
//因JSON返回遇到了 强类型的矛盾  暂时无法处理
//状态码|内容/错误信息
func (this *Context) TxtReturn(code int64, format string, a ...interface{}) {
	msg := ""

	if strings.Contains(format, "%s") || strings.Contains(format, "%d") || strings.Contains(format, "%v") || strings.Contains(format, "%t") {
		msg = fmt.Sprintf(format, a)
	} else {
		msg = format
	}

	str := strconv.FormatInt(code, 10) + "|" + msg

	fmt.Println(str)

	_, _ = fmt.Fprintf(this.Writer, str)
}
