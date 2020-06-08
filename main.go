package main

import (
	"flag"
	"fmt"
	"github.com/solaa51/gosab/src/controller"
	"github.com/solaa51/gosab/system/core/app"
	"github.com/solaa51/gosab/system/core/myContext"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

//全局APP设置信息
var APP *app.App

//包含-D参数则 进入守护进程
var d = flag.Bool("d", false, "启动守护进程-d")
var g = flag.Bool("g", false, "平滑重启-g，必须要有服务已经运行中") //系统自动调用

type MyHandler struct{}

//TODO 新增加的控制器 需要在此处添加处理
func (h *MyHandler) configClass(ctx *myContext.Context) {
	var class interface{} //初始化一个interface 用于接收实例化的struct的地址

	switch ctx.Controller {
	case "welcome":
		class = &controller.Welcome{Ctx: ctx}
	default:
		http.Error(ctx.Writer, "碰到了不认识的路由", http.StatusNotFound)
		return
	}

	h.run(ctx, class)
}

func (h *MyHandler) run(ctx *myContext.Context, class interface{}) {
	getType := reflect.TypeOf(class)
	getValue := reflect.ValueOf(class)

	_, bol := getType.MethodByName(ctx.Method)
	if !bol {
		_, _ = fmt.Fprint(ctx.Writer, "碰到了不认识的路径")
		return
	}

	//发起调用
	methodValue := getValue.MethodByName(ctx.Method)
	args := make([]reflect.Value, 0)

	start := time.Now()
	_ = methodValue.Call(args) //执行方法
	//计算出执行时间 记录调用日志
	APP.Log.Trace("run time:" + ctx.Controller + "/" + ctx.Method + "--" + time.Since(start).String())
}

func (h *MyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//处理静态文件请求
	if r.URL.String() == "/favicon.ico" {
		http.ServeFile(w, r, "./favicon.ico")
		return
	}

	//处理配置中允许的静态文件映射
	for _, fp := range APP.StaticFiles {
		if strings.HasPrefix(r.URL.String(), fp.Prefix) {
			s, _ := url.QueryUnescape(r.URL.String()) //url转义
			ff := fp.Path + strings.Replace(s, fp.Prefix, "", 1)
			if strings.HasSuffix(ff, "/") {
				ff = ff[:(len(ff) - 1)]
			}

			fi, err := os.Stat(ff)
			if err != nil {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}

			if !fp.Dir { //不允许遍历目录内容
				if fi.IsDir() {
					http.Error(w, "不允许遍历文件", http.StatusNotImplemented)
					return
				}
			}

			http.ServeFile(w, r, ff)
			return
		}
	}

	//动态匹配路由
	ctx, err := myContext.NewContext(r, w, APP) //解析请求 构建上下文
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	h.configClass(ctx)
}

func main() {
	flag.Parse()

	daemon(*d)

	//清理资源
	defer APP.Close()

	//http handle 处理
	handler := MyHandler{}

	//新的
	err := APP.Start(&handler, *g)
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	APP = app.NewApp("")
}

//进入守护进程
func daemon(d bool) {
	if d && os.Getppid() != 1 { //判断父进程  父进程为1则表示已被系统接管
		filePath, _ := filepath.Abs(os.Args[0]) //将启动命令 转换为 绝对地址命令
		cmd := exec.Command(filePath, os.Args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_ = cmd.Start()

		os.Exit(0)
	}
}
