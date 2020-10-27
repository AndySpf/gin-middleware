package middleware

import (
	"github.com/gin-gonic/gin"
	"sync/atomic"
	"time"
)

var rule = map[time.Duration]int32{
	time.Minute * 10: 200,
}

type ipLimit struct {
	curIndex int32
	maxCount int32
	ipMap    []map[string]int
	ticker   *time.Ticker
}

var limit ipLimit

func initLimit() {
	// 每个请求休眠一段时间接受下一个请求?
	ticker := time.NewTicker(time.Minute) // 定期更换新的map， 或者每一次到时间后标志位改变，map逆向计数
	limit = ipLimit{
		curIndex: 0,
		maxCount: 5,
		ipMap:    make([]map[string]int, 16),
		ticker:   ticker,
	}
}

func IpLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		index := int(atomic.LoadInt32(&limit.curIndex))
		if _, ok := limit.ipMap[index][ip]; !ok {
			limit.ipMap[index][ip] = 0
		}
		if limit.ipMap[index][ip] < int(limit.maxCount) {
			limit.ipMap[index][ip]++
			c.Next()
		} else {
			c.Writer.WriteString("connect refused")
			c.Abort()
		}
	}
}

func (p *ipLimit) start() {
	for {
		select {
		case <-p.ticker.C:
			oldIndex := atomic.LoadInt32(&p.curIndex)
			if oldIndex+1 == int32(len(p.ipMap)) {
				atomic.StoreInt32(&p.curIndex, 0)
			} else {
				atomic.AddInt32(&p.curIndex, 1)
			}
			p.ipMap[oldIndex] = make(map[string]int)
		}
	}
}
