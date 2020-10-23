package middleware

import (
	"github.com/gin-gonic/gin"
	"sync"
	"sync/atomic"
	"time"
)

type circleTiming struct {
	curIndex int32
	mapCount int32
	ticker   *time.Ticker
	value    []*pathMap
	request  chan string
	do       func(value []QPSInfo, total int)
}

type pathMap struct {
	qps map[string]int
}

var (
	once          sync.Once
	qpsStatistics *circleTiming
)

// qpsall 初始化qps统计器
func QPSInit(interval time.Duration, doReport func(value []QPSInfo, total int)) {
	once.Do(func() {
		qpsStatistics = newDefaultCircle(interval, doReport)
		go qpsStatistics.start()

		initLimit()
		go limit.start()
	})
}

func QPS() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Request.Method + "|" + c.Request.URL.Path
		qpsStatistics.request <- key
		c.Next()
	}
}

func newDefaultCircle(interval time.Duration, doReport func(value []QPSInfo, total int)) *circleTiming {
	paths := make([]*pathMap, 16)
	ticker := time.NewTicker(interval)
	for i := range paths {
		paths[i] = &pathMap{
			qps: map[string]int{},
		}
	}
	return &circleTiming{
		ticker:   ticker,
		mapCount: int32(len(paths)),
		value:    paths,
		request:  make(chan string, 1000),
		do:       doReport,
	}
}

func (p *circleTiming) start() {
	for {
		select {
		case <-p.ticker.C:
			oldIndex := atomic.LoadInt32(&p.curIndex)
			if oldIndex+1 == p.mapCount {
				atomic.StoreInt32(&p.curIndex, 0)
			} else {
				atomic.AddInt32(&p.curIndex, 1)
			}
			go p.report(int(oldIndex))
		case path := <-p.request:
			index := atomic.LoadInt32(&qpsStatistics.curIndex)
			if _, ok := qpsStatistics.value[index].qps[path]; !ok {
				qpsStatistics.value[index].qps[path] = 0
			}
			qpsStatistics.value[index].qps[path]++
		}
	}
}

// QPSInfo 各个请求路径的qps归纳信息
type QPSInfo struct {
	Path  string
	Count int
}

func (p *circleTiming) report(index int) {
	// mapCount * ticker 时间内清理不完，可能会和start函数中产生冲突
	info := make([]QPSInfo, 0, len(p.value[index].qps))
	total := 0
	for key := range p.value[index].qps {
		info = append(info, QPSInfo{
			Path:  key,
			Count: p.value[index].qps[key],
		})
		total += p.value[index].qps[key]
	}

	p.do(info, total)

	p.value[index].qps = map[string]int{}
}
