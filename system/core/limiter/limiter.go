package limiter

//流控方案
type ConnLimiter struct {
	conCurrentCon int      //当前可用连接数
	bucket        chan int //池子
}

//限流器
func NewLimiter(num int) *ConnLimiter {
	return &ConnLimiter{
		conCurrentCon: num,
		bucket:        make(chan int, num),
	}
}

func (cl *ConnLimiter) GetConn() bool {
	if len(cl.bucket) >= cl.conCurrentCon {
		return false
	}

	cl.bucket <- 1
	return true
}

func (cl *ConnLimiter) ReleaseConn() {
	<-cl.bucket
}
