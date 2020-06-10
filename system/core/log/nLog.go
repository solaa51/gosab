package log

import (
	"fmt"
	"github.com/solaa51/gosab/system/core/commonFunc"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

/**
日志记录
*/
type NLog struct {
	LPrefix string //日志文件前缀
	Path    string

	Env string //设置环境 本地和测试时  打印到控制台，线上环境则只输出到文件

	muFile *sync.RWMutex //文件锁

	logFile     *os.File //日志文件
	logDateName string   //日志文件的日期  变更时修改logFile
}

func init() {
	//判断logs文件夹是否存在
	_, err := os.Stat("logs")
	if err != nil {
		if os.IsNotExist(err) {
			//创建文件夹
			if err := os.Mkdir("logs", os.ModePerm); err != nil {
				log.Fatal("创建日志文件夹失败", err)
				return
			}
		} else {
			log.Fatal("检查日志文件夹出错", err)
			return
		}
	}
}

func NewLog(env string, prefix string) *NLog {
	l := NLog{}
	l.LPrefix = prefix
	l.muFile = new(sync.RWMutex)
	l.Path = commonFunc.GetAppDir()
	l.Env = env

	dd := commonFunc.Date("Y-m-d", 0)

	l.logDateName = l.LPrefix + dd
	logFile := l.Path + "logs/" + l.logDateName + ".log"

	var err error
	l.logFile, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("打开日志文件时发生错误")
	}

	//定时处理日志文件 判断时间 关闭原有句柄 并重新赋值文件句柄
	go func() {
		t := time.NewTicker(time.Second * 10)
		defer t.Stop()
		for {
			<-t.C
			dd := commonFunc.Date("Y-m-d", 0)
			logName := l.LPrefix + dd
			if logName != l.logDateName {
				fmt.Println("修改日志文件句柄" + l.logDateName + " => " + logName)
				l.muFile.RLock()
				l.logFile.Close() //关闭原来文件句柄
				logFile := l.Path + "logs/" + l.LPrefix + l.logDateName + ".log"
				l.logFile, _ = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				l.logDateName = logName
				l.muFile.RUnlock()
			}
		}
	}()

	return &l
}

func (this *NLog) Close() {
	this.logFile.Close()
}

// "TRACE", "DEBUG", "INFO", "WARN", "ERROR"
func (this *NLog) Info(s string) {
	this.echo("INFO", s)
}

func (this *NLog) Warn(s string) {
	this.echo("WARN", s)
}

func (this *NLog) Error(s string) {
	this.echo("ERROR", s)
}

func (this *NLog) Trace(s string) {
	this.echo("TRACE", s)
}

func (this *NLog) Debug(s string) {
	this.echo("DEBUG", s)
}

func (this *NLog) echo(prefix, s string) {
	this.muFile.Lock()
	defer this.muFile.Unlock()

	//| log.Lshortfile 输出的是 当前出错输出内容的行 没什么意义
	var pc uintptr
	var fileName string
	var line int
	var funcName string

	pc, fileName, line, _ = runtime.Caller(2)
	funcName = runtime.FuncForPC(pc).Name()
	if strings.HasSuffix(funcName, "myContext.NewContext") {
		pc, fileName, line, _ = runtime.Caller(3)
		funcName = runtime.FuncForPC(pc).Name()
	}

	fileName = filepath.Base(fileName)

	nPrefix := prefix + " " + fileName + " " + funcName + " " + strconv.Itoa(line)

	//空环境 本地 测试 则也在标准输出 输出内容
	var logIO io.Writer
	if this.Env == "" || this.Env == "local" || this.Env == "test" {
		logIO = io.MultiWriter(os.Stdout, this.logFile)
	} else {
		logIO = this.logFile
	}

	logger := log.New(logIO, nPrefix+": ", log.Ldate|log.Ltime)
	logger.Print(s)
}
