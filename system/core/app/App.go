package app

import (
	"gosab/system/core/commonFunc"
	"gosab/system/core/configFileMonitor"
	"gosab/system/core/graceful"
	slog "gosab/system/core/log"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
)

type App struct {
	Name string `toml:"name"` //应用名称

	//TODO 数据库配置

	HTTP     bool   `toml:"http"`  //是否开启http服务
	PORT     string `toml:"PORT"`  //http 监听端口
	HTTPS    bool   `toml:"https"` //是否开启https服务
	HTTPSKEY string `toml:"httpsKey"`
	HTTPSPEM string `toml:"httpsPem"`

	ENV string `toml:"env"` //表示当前环境 本地local  发布dev   测试test

	SIGNCHECK bool   `toml:"signCheck"` //是否验证签名  总开关
	NSIGN     string `toml:"nSign"`     //不验证签名的class访问 多个,隔开

	IPCHECK bool   `toml:"ipCheck"` //是否校验ip
	GIPS    string `toml:"gIps"`    //允许通过的ip
	NIPS    string `toml:"nIps"`    //不需要校验的ip 多个,隔开

	StaticFiles []StaticFile `toml:"staticFiles"` //允许遍历的静态文件映射目录

	/******以下为自动判断 生成配置******/
	HOMEDIR   string //程序体文件所在目录  入口目录
	CONFIGDIR string //程序配置文件所在目录

	Log *slog.NLog //用于 记录日志
}

//静态文件映射关系
type StaticFile struct {
	Prefix string `toml:"prefix"` //识别前缀
	Path   string `toml:"path"`   //对应的服务器绝对地址
	Dir    bool   `toml:"dir"`    //是否允许遍历目录下所有文件
}

//新版 启动服务
func (this *App) Start(handler http.Handler, gracefulReload bool) error {
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	httpsPem := ""
	httpsKey := ""
	if this.HTTPS {
		//检测https配置的密钥文件是否存在
		_, err := os.Stat(this.CONFIGDIR + this.HTTPSPEM)
		if err != nil {
			return err
		}

		_, err = os.Stat(this.CONFIGDIR + this.HTTPSKEY)
		if err != nil {
			return err
		}

		httpsPem = this.CONFIGDIR + this.HTTPSPEM
		httpsKey = this.CONFIGDIR + this.HTTPSKEY
	}

	return graceful.Start(":"+this.PORT, this.Log, mux, httpsPem, httpsKey, gracefulReload)
}

//判断class是否能通过ip检查
func (this *App) IpClass(cName string, ip string) bool {
	if cName == "" {
		return false
	}

	if !this.IPCHECK == true {
		return true
	}

	//内网ip 放过
	if commonFunc.InnerIP(ip) {
		return true
	}

	//查看 class 是否为不需要检测
	if this.NIPS != "" {
		names := strings.Split(this.NIPS, ",")
		for _, v := range names {
			if v == cName {
				return true
			}
		}
	}

	if this.NIPS != "" {
		ips := strings.Split(this.GIPS, ",")
		for _, v := range ips {
			if v == ip {
				return true
			}
		}
	}

	return false
}

//初始化APP设置 并启动http服务
func NewApp(appConfig string) *App {
	myApp := &App{}

	var configFile string
	if appConfig != "" {
		configFile = appConfig
	} else {
		configFile = "app.toml"
	}

	if !strings.HasSuffix(configFile, ".toml") {
		log.Fatal("仅支持toml类型的配置文件")
	}

	path, _ := commonFunc.FindConfigPath(configFile)
	_, err := toml.DecodeFile(path+configFile, myApp)
	if err != nil {
		log.Fatal("无法解析配置文件app.toml", err)
	}

	//检测端口是否被占用 TODO 其他方法检测吧
	if myApp.HTTP {
		if myApp.PORT == "" {
			log.Fatal("请配置要使用的端口号")
		}
	}

	myApp.HOMEDIR = commonFunc.GetAppDir()
	myApp.CONFIGDIR, _ = commonFunc.FindConfigPath(configFile)

	//初始化自定义的log库
	myApp.Log = slog.NewLog(myApp.ENV, "")

	//fmt.Println(myApp)
	//检测配置文件修改 则修改APP设置
	_, _ = configFileMonitor.NewConFile(configFile, func(interface{}) {
		resetAppConfig(myApp, configFile)
	})

	//TODO 有更新文件 时间一直变动 bug  处理升级机制
	if myApp.HTTP {
		go myApp.hasNewKillSelf()
	}

	return myApp
}

func resetAppConfig(app *App, configFile string) {
	myTmpApp := &App{}
	path, _ := commonFunc.FindConfigPath(configFile)
	_, _ = toml.DecodeFile(path+configFile, myTmpApp)

	app.ENV = myTmpApp.ENV
	app.IPCHECK = myTmpApp.IPCHECK
	app.NIPS = myTmpApp.NIPS
	app.GIPS = myTmpApp.GIPS

	app.SIGNCHECK = myTmpApp.SIGNCHECK
	app.NSIGN = myTmpApp.NSIGN

	app.StaticFiles = myTmpApp.StaticFiles
}

//关闭日志资源
func (this *App) Close() {
	this.Log.Close()
}

//检测当前环境下 可执行文件是否有更新，如果存在更新 则 给自己发送升级信号
func (this *App) hasNewKillSelf() {
	a, _ := filepath.Abs(os.Args[0])
	af, _ := os.Stat(a)
	aLT := af.ModTime().Unix()
	go func() {
		t := time.NewTicker(time.Second * 10)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				an, err := os.Stat(a)
				if err != nil {
					//此时可能文件正在更新，需要跳过，因为此时的文件，可能是不完整的
					continue
				}
				if an.ModTime().Unix() > aLT { //存在更新
					p, err := os.FindProcess(os.Getpid())
					if err != nil {
						this.Log.Info("获取进程pid失败：" + err.Error())
						continue
					}

					//发送信号
					this.Log.Info("发送升级信号")
					_ = p.Signal(syscall.SIGHUP)
				}
			}
		}
	}()
}
