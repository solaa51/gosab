package graceful

import (
	"context"
	"errors"
	"fmt"
	slog "github.com/solaa51/gosab/system/core/log"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

/**
用于启动http服务和支持热重启
*/

type graceful struct {
	Server   *http.Server //http服务server配置实例
	Listener net.Listener
	Log      *slog.NLog //用于 记录日志

	HttpsPem string //https ssl配置
	HttpsKey string //https ssl配置
}

func (this *graceful) start() {
	//将http服务放到一个goroutine中
	//不能将http服务放到 main goroutine中 给接收信号让开路
	go func() {
		var err error
		if this.HttpsPem != "" && this.HttpsKey != "" {
			err = this.Server.ServeTLS(this.Listener, this.HttpsPem, this.HttpsKey)
		} else {
			err = this.Server.Serve(this.Listener)
		}

		if err != nil {
			this.Log.Error("启动http服务失败：" + err.Error())
			return
		}
	}()

	//服务已启动
	xx := fmt.Sprintf("服务进程为：%d, 您可用\nkill -HUP %d, 重启或升级服务\n", os.Getpid(), os.Getpid())
	this.Log.Info(xx)

	this.singleHandle()

	return
}

//重启服务
func (this *graceful) restart() error {
	ln, ok := this.Listener.(*net.TCPListener)
	if !ok {
		return errors.New("转换tcp listener失败")
	}

	ff, err := ln.File()
	if err != nil {
		return errors.New("获取socket文件描述符失败")
	}

	cmd := exec.Command(os.Args[0], []string{"-g"}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{ff} //重用原有的socket文件描述符

	err = cmd.Start()
	if err != nil {
		return errors.New("启动新进程报错了：" + err.Error())
	}

	return nil
}

//监听主进程的信号
func (this *graceful) singleHandle() {
	//创建一个无阻塞信号 channel
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-ch
		ctx, _ := context.WithTimeout(context.Background(), time.Second*20)
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			this.Log.Info("关闭服务")
			signal.Stop(ch)
			_ = this.Server.Shutdown(ctx) //平滑关闭原有连接
			return
		case syscall.SIGHUP:
			this.Log.Info("热重启服务启动")
			fmt.Println("收到信号：", sig)
			err := this.restart()
			if err != nil {
				this.Log.Error("热重启服务失败" + err.Error())
				log.Fatal("热重启服务失败", err)
			}

			_ = this.Server.Shutdown(ctx) //平滑关闭原有连接
			this.Log.Info("热重启完成")
			return
		}
	}
}

//启动
func Start(addr string, log *slog.NLog, mux http.Handler, httpsPem string, httpsKey string, gracefulReload bool) error {
	var ln net.Listener
	var err error
	if gracefulReload { //启动命令中包含参数 热重启时，从socket文件描述符 重新启动一个监听
		//当存在监听socket时 socket的文件描述符就是3 所以从本进程的3号文件描述符 恢复socket监听
		f := os.NewFile(3, "")
		ln, err = net.FileListener(f)
		if err != nil {
			return err
		}
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         nil,
		ReadTimeout:       time.Second * 30, //读取包括请求体的整个请求的最大时长
		WriteTimeout:      time.Second * 30, //写响应允许的最大时长 30秒程序未能输出 则退出http连接
		IdleTimeout:       time.Second * 30, //当开启了保持活动状态（keep-alive）时允许的最大空闲时间
		ReadHeaderTimeout: time.Second * 2,  //允许读请求头的最大时长
	}

	gf := &graceful{
		Log:      log,
		Server:   server,
		Listener: ln,
		HttpsPem: httpsPem,
		HttpsKey: httpsKey,
	}

	gf.start()

	return nil
}
