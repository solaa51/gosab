package commonFunc

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gopkg.in/yaml.v2"
	"hash/crc32"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const TIME_STR = "2006-01-02 15:04:05"
const TIME_STRS = "2006-01-02 15:04"

/**
存储公共方法
*/
//查找配置文件路径
//大概有三种方式
//1. shell所在即为执行程序所在目录
//2. 相对路径调用的形式[.main]
//3. 绝对路径调用的形式[/data/bin/main]
//返回配置文件的路径 不包含文件名 这样可以放到全局去给所有程序调用
func FindConfigPath(fi string) (string, error) {
	homePath := GetAppDir()

	//列出 本框架内需要的几种配置文件的目录结构
	mP1 := homePath + "config/" //正常情况 可执行文件与配置在同一级目录
	mPath1 := homePath + "config/" + fi

	mP2 := homePath + "../../../config/" //cli程序 调试期间使用的目录布局
	mPath2 := homePath + "../../../config/" + fi

	mP3 := homePath + "../config/" //工具类cli程序 调试期间使用的目录布局
	mPath3 := homePath + "../config/" + fi

	/*path1, _ := os.Getwd()
	fmt.Println(path1)
	fmt.Println(mP1, mP2, mP3)*/

	_, err := os.Stat(mPath1)
	if err == nil {
		return mP1, nil
	}

	_, err = os.Stat(mPath2)
	if err == nil {
		return mP2, nil
	}

	_, err = os.Stat(mPath3)
	if err == nil {
		return mP3, nil
	}

	return "", errors.New("找不到配置文件: " + fi)
}

//获取到可执行文件的绝对地址
//调试环境下返回 源码的路径
//正式环境下返回 可执行文件的路径
//区分 go run 下执行 还是 go build 之后的可执行文件 按系统区分
func GetAppDir() string {
	var dir string
	dir, err := filepath.Abs(filepath.Dir(os.Args[0])) //返回绝对路径  filepath.Dir(os.Args[0])去除最后一个元素的路径
	if err != nil {
		log.Fatal(err)
	}

	//如果路径中 含有 /T/go-build 字符 则可认为是 go run 下执行的临时程序
	switch runtime.GOOS {
	case "darwin":
		if strings.Contains(dir, "/T/go-build") {
			dir, _ = os.Getwd()
		}

		return dir + string(os.PathSeparator)
	case "windows":
		if strings.Contains(dir, "\\Temp\\go-build") {
			dir, _ = os.Getwd()
		}
		return dir + string(os.PathSeparator)
	default:
	}

	//return strings.Replace(dir, "\\", "/", -1) + "/" //将\替换成/
	return dir + string(os.PathSeparator)
}

//加载配置文件
//fi 配置文件名
//st 待解析的结构体(地址)
//返回 配置文件路径不包含文件名  错误
func LoadConfig(fi string, st interface{}) (string, error) {
	cf, err := FindConfigPath(fi)
	if err != nil {
		return cf, err
	}

	if strings.HasSuffix(fi, ".toml") {
		_, err := toml.DecodeFile(cf+fi, st)
		if err != nil {
			log.Fatal("无法解析配置文件app.toml", err)
		}
	} else {
		configData, err := ioutil.ReadFile(cf + fi)
		if err != nil {
			return "", errors.New("读取配置文件内容时出错" + err.Error())
		}

		err = yaml.Unmarshal(configData, st)
		if err != nil {
			return "", errors.New("解析配置文件出错" + err.Error())
		}
	}

	return cf, nil
}

// ClientIP 尽最大努力实现获取客户端 IP 的算法。
// 解析 X-Real-IP 和 X-Forwarded-For 以便于反向代理（nginx 或 haproxy）可以正常工作。
func ClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	ip := strings.TrimSpace(strings.Split(xForwardedFor, ",")[0])
	if ip != "" {
		return ip
	}

	ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	if ip != "" {
		return ip
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

//判断是否为内网ip
func InnerIP(ip string) bool {
	if ip == "::1" { //本机
		return true
	} else if strings.HasPrefix(ip, "192.168.") { //内网地址
		return true
	}

	return false
}

func Md5(s string) string {
	md5Str := md5.New()
	md5Str.Write([]byte(s))

	return hex.EncodeToString(md5Str.Sum(nil))
}

//时间函数 参考PHP返回 Go的记起来有点累
func Time() int64 {
	return time.Now().Unix()
}

//支持最常用的 Y-m-d H:i:s
//2006-01-02 15:04:05
//stamp 时间戳 如果为0则处理为当前时间
func Date(phpFormat string, stamp int64) string {
	var st time.Time
	if stamp == 0 {
		st = time.Now()
	} else {
		st = time.Unix(stamp, 0)
	}

	switch phpFormat {
	case "Y": //年
		return strconv.Itoa(st.Year())
	case "m", "n": //月
		return strconv.Itoa(int(st.Month()))
	case "d", "j": //日
		return strconv.Itoa(st.Day())
	case "H": //时
		return strconv.Itoa(st.Hour())
	case "i": //分
		return strconv.Itoa(st.Minute())
	case "s": //秒
		return strconv.Itoa(st.Second())
	case "Y-m": //年月
		return strconv.Itoa(st.Year()) + "-" + strconv.Itoa(int(st.Month()))
	case "Y-m-d":
		return st.Format(strings.Split(TIME_STR, " ")[0])
	case "Y-m-d H:i:s":
		return st.Format(TIME_STR)
	case "Y-m-d H:i":
		return st.Format(TIME_STRS)
	case "Y-m-dTH:i:sZ": //UTC时间  T Z格式使用
		s := st.UTC().String()
		return s[:10] + "T" + s[11:19] + "Z"
	default:
		return ""
	}
}

//将时间戳 转换为 指定时间格式 对应的 时间戳
//仅支持最常用的 Y-m-d H:i:s
//仅支持最常用的 Y-m-d
//stamp 时间戳 如果为0则处理为当前时间
func StrToTime(phpFormat string, stamp int64) int64 {
	var st time.Time
	if stamp == 0 {
		st = time.Now()
	} else {
		st = time.Unix(stamp, 0)
	}

	var format string
	var str string
	switch phpFormat {
	case "Y-m-d":
		format = strings.Split(TIME_STR, " ")[0]
	case "Y-m-d H:i:s":
		format = TIME_STR
	default:
		return 0
	}

	str = st.Format(format)
	timeArea, _ := time.LoadLocation("Asia/Shanghai")
	tt, _ := time.ParseInLocation(format, str, timeArea)
	return tt.Unix()
}

//将时间字符串 转换为 时间戳
func TimeStrToTime(str string) int64 {
	format := TIME_STR
	timeArea, _ := time.LoadLocation("Asia/Shanghai")
	tt, _ := time.ParseInLocation(format, str, timeArea)
	return tt.Unix()
}

//只支持一级 生成url 查询字符串
func SortBuildQuery(data map[string]interface{}) string {
	key := make([]string, len(data))
	for k, _ := range data {
		key = append(key, k)
	}

	sort.Strings(key)

	str := ""
	for k, v := range key {
		if k > 0 {
			str += "&"
		}

		str += url.QueryEscape(v) + "=" + url.QueryEscape(interfaceToString(data[v]))
	}

	return str
}

//只支持一级 生成url 查询字符串
func SortBuildQueryStr(data map[string]string) string {
	key := make([]string, 0)
	for k, _ := range data {
		key = append(key, k)
	}

	sort.Strings(key)

	str := ""
	for k, v := range key {
		if k > 0 {
			str += "&"
		}

		str += url.QueryEscape(v) + "=" + url.QueryEscape(data[v])
	}

	return str
}

//任意简单类型转字符串
func interfaceToString(v interface{}) string {
	var ps string
	if v != nil {
		switch v.(type) {
		case int:
			ps = strconv.Itoa(v.(int))
		case int64:
			ps = strconv.FormatInt(v.(int64), 10)
		case string:
			ps = v.(string)
		case float64:
			ps = fmt.Sprintf("%.2f", v.(float64))
		default:
			ps = ""
		}
	} else {
		ps = ""
	}

	return ps
}

//获取本地IPv4地址
func LocalIPV4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

//获取外网IP
func InternetIp() string {
	ip, _ := GetPost("GET", "http://ip.cip.cc", nil, nil, nil)
	return strings.TrimSpace(ip)
}

/**
发送get 或 post请求 获取数据
*/
func GetPost(method string, sUrl string, data map[string]string, head map[string]string, cookie []*http.Cookie) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { //禁止自动跳转
			return http.ErrUseLastResponse
		},
	}

	//请求体数据
	var postBody *strings.Reader
	if data != nil {
		pData := url.Values{}
		for k, v := range data {
			pData.Add(k, v)
		}
		postBody = strings.NewReader(pData.Encode())
	} else {
		postBody = strings.NewReader("")
	}

	req, err := http.NewRequest(method, sUrl, postBody)
	if err != nil {
		return "", err
	}

	if _, ok := head["User-Agent"]; !ok {
		req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.120 Safari/537.36")
	}
	if _, ok := head["Content-Type"]; !ok {
		req.Header.Add("User-Agent", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if head != nil {
		for k, v := range head {
			if v != "" {
				req.Header.Add(k, v)
			}
		}
	}

	if cookie != nil {
		for _, c := range cookie {
			req.AddCookie(c)
		}
	}

	response, err := client.Do(req)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return string(body), nil
}

/**
发送get 或 post请求 获取数据 返回response和error
*/
func GetPostRequest(method string, sUrl string, data map[string]string, head map[string]string, cookie []*http.Cookie, redirect bool) (*http.Response, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { //禁止自动跳转
			if redirect {
				return nil
			} else {
				return http.ErrUseLastResponse
			}
		},
	}

	//请求体数据
	var postBody *strings.Reader
	if data != nil {
		pData := url.Values{}
		for k, v := range data {
			pData.Add(k, v)
		}
		postBody = strings.NewReader(pData.Encode())
	} else {
		postBody = strings.NewReader("")
	}

	req, err := http.NewRequest(method, sUrl, postBody)
	if err != nil {
		return nil, err
	}

	if _, ok := head["User-Agent"]; !ok {
		req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.120 Safari/537.36")
	}
	if _, ok := head["Content-Type"]; !ok {
		req.Header.Add("User-Agent", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if head != nil {
		for k, v := range head {
			if v != "" {
				req.Header.Add(k, v)
			}
		}
	}

	if cookie != nil {
		for _, c := range cookie {
			req.AddCookie(c)
		}
	}

	return client.Do(req)
}

//utf8编码 转 gbk编码
func UTF82GBK(str []byte) ([]byte, error) {
	r := transform.NewReader(bytes.NewReader(str), simplifiedchinese.GBK.NewEncoder())
	b, err := ioutil.ReadAll(r)
	return b, err
}

//gbk编码 转 utf8编码
func GBK2UTF8(str []byte) ([]byte, error) {
	r := transform.NewReader(bytes.NewReader(str), simplifiedchinese.GBK.NewDecoder())
	b, err := ioutil.ReadAll(r)
	return b, err
}

//给数据库分表计算
func Mod(id int64) int64 {
	str := strconv.FormatInt(id, 10)
	shu := crc32.ChecksumIEEE([]byte(str))

	return int64(math.Mod(float64(shu), 10))
}

//生成随机数[n - m)
func RandInt(start, end int64) (int64, error) {
	if end < start {
		return 0, errors.New("结束位置必须大于开始位置")
	}

	if end == start {
		return start, nil
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(end-start))

	//rand.Seed(time.Now().UnixNano())
	//return rand.Int63n(end-start) + start, nil

	return n.Int64(), nil
}
