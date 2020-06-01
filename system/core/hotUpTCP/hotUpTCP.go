package hotUpTCP

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

/*
实现web服务的热更新 返回 更新的结构 包含 *net.Listener
 */

type HotUpSocket struct {
	socket *net.TCPListener //tcp监听器
}

func (h *HotUpSocket) stop() {
	_ = h.socket.SetDeadline(time.Now())
}

//启动一个可热升级的http服务
//param addr 服务器监听地址
//param handler http请求处理器
//TODO 还应该处理掉 上下文
//TODO 还应该处理掉 日志文件
func NewHttpServer(addr string, mux *http.ServeMux) error {
	var fd *net.TCPListener

	if mux == nil {
		fmt.Print("http路由处理为空, 启用默认路由处理\n")

		mux = http.NewServeMux()
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			_, _ = fmt.Fprint(writer, "hello world!")
		})
	}

	//检查环境变量是否设置重启
	if os.Getenv("__tcp__reloadUP__") == "true" {
		//window系统是否不同？ c:\\temp\
		file := os.NewFile(3, "/tmp/sTCPReUP")
		listener, err := net.FileListener(file)
		if err != nil {
			_ = fmt.Errorf("监听服务启动失败:%s \n", err.Error())
			os.Exit(0)
		}

		//将监听断言为 tcp监听器的文件描述符
		fd = listener.(*net.TCPListener)
	}else{
		//配置http服务的监听config
		lC := net.ListenConfig{}
		listener, err := lC.Listen(context.Background(), "tcp", addr)
		if err != nil {
			_ = fmt.Errorf("监听服务启动失败:%s \n", err.Error())
			os.Exit(0)
		}

		//将监听断言为 tcp监听器的文件描述符
		fd = listener.(*net.TCPListener)
	}

	ss := &HotUpSocket{socket:fd}

	go ss.listenSignal()

	fmt.Printf("服务进程为：%d, 您可用\nkill -HUP %d, 重启或升级服务\n端口为: %s\n\n", os.Getpid(), os.Getpid(), addr)

	serve := http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         nil,
		ReadTimeout:       time.Second * 30,
		ReadHeaderTimeout: 0,
		WriteTimeout:      0,
		IdleTimeout:       0,
		MaxHeaderBytes:    0,
		TLSNextProto:      nil,
		ConnState:         nil,
		ErrorLog:          nil,
	}

	err := serve.Serve(keepAliveListen{fd})
	if err != nil {
		return errors.New("启动http服务失败：" + err.Error())
	}

	return nil
}

//监听服务器给予的信号
func (h *HotUpSocket) listenSignal() {

	//创建一个阻塞信号 channel
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM)
	for sig := range signals {
		if sig == syscall.SIGTERM{ //可接收处理信号的 关闭信号 kill pid命令
			h.stop() //关掉socket io文件
			// 理论会中断掉 接下来的所有请求
			// 但是在本实例中 不生效
			// 客户端连接依然能进来 但是请求不到任何数据 会被丢到 keepalive的3分钟等待中去

			//log.Printf("当前还有%d个请求正在处理中\n", socketServe.Count)

			os.Exit(0)
		}else if sig == syscall.SIGHUP { //挂起信号
			h.stop() //关掉socket io文件

			//获取当前进程的 socket文件描述符
			ff, err := h.socket.File()
			if err != nil {
				log.Println("获取socket文件描述符失败")
				os.Exit(999)
			}

			//为即将启动的进程 初始化信息
			execSpc := &os.ProcAttr{
				Dir:   "",
				Env: os.Environ(), //设置环境变量
				Files:[]*os.File{
					os.Stdin,
					os.Stdout,
					os.Stderr,
					ff, //重用原有的socket文件描述符
				},
				Sys:   nil,
			}

			process, err := os.StartProcess(os.Args[0], os.Args, execSpc )
			if err != nil {
				log.Fatalln("创建新进程失败：", err)
			}
			pid := process.Pid

			log.Println("收到了sighup信号：创建新的进程为：", pid)

			log.Println(os.Getpid(), "服务端正常关闭成功")

			//关掉 原有的服务， 新的服务已经创建并接收新连接
			os.Exit(0)
		}
	}
}

//为热升级 照抄了http包里的结构
type keepAliveListen struct {
	*net.TCPListener
}
func (ln keepAliveListen) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}