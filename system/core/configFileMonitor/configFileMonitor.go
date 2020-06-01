package configFileMonitor

import (
	"bufio"
	"gosab/system/core/commonFunc"
	"log"
	"os"
	"sync"
	"time"
)

/**
监控文件变化，当文件变化时 执行预设的操作
每秒查看一次文件
使用情景：
	数据库配置变更，则重启服务

使用方法:
   _, _ = NewConfFile("conf/app.conf", func(param interface{}) {
		p, ok := param.(string)
		if !ok {
			fmt.Println("参数类型错误")
			os.Exit(2)
		}
		fmt.Println(p)
	})
*/

//配置文件 结构
type confModify struct {
	modTime int64  //文件修改时间
	content []byte //文件内容
	path    string
	m       sync.Mutex
}

//改进版检测 参数为配置文件名
func NewConFile(name string, pf func(interface{})) (*confModify, error) {
	//查找配置文件
	filePath, err := commonFunc.FindConfigPath(name)
	if err != nil {
		return nil, err
	}

	conf := &confModify{
		path: filePath + name,
	}

	go func() {
		for {
			conf.ListenModify(pf)
			time.Sleep(1 * time.Second)
		}
	}()

	return conf, nil
}

//监控文件状态 变化时 执行预设函数
func (c *confModify) ListenModify(pf func(interface{})) {
	c.m.Lock()
	file, err := os.Open(c.path)
	if err != nil {
		log.Fatal("获取文件出错")
	}

	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatal("获取文件基本信息出错")
	}

	if fileInfo.ModTime().Unix() != c.modTime {
		c.modTime = fileInfo.ModTime().Unix()

		//调用参数 看情况 传递 本处为返回文件内容
		//如果文件内容大 则最好 在自定义函数中自己处理
		fr := bufio.NewReader(file)
		b2 := make([]byte, fileInfo.Size())
		_, _ = fr.Read(b2)
		c.content = b2
		pf(string(b2)) //调用函数
	}

	c.m.Unlock()
}
