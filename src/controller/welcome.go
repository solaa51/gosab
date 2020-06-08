package controller

import (
	"fmt"
	"github.com/solaa51/gosab/system/core/myContext"
)

type Welcome struct {
	Ctx *myContext.Context
}

func (this *Welcome) Index() {
	this.Ctx.JsonReturn(0, "新版上线了", "")
}

func (this *Welcome) Wak() {
	this.Ctx.JsonReturn(0, "另一个方法", "")
}

func (this *Welcome) memda() {
	_, _ = fmt.Fprint(this.Ctx.Writer, "终于好了")
}
