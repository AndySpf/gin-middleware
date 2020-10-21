package middleware

import (
	"github.com/gin-gonic/gin"
	"sync"
	"sync/atomic"
	"time"
)

type circleTiming struct {
	ticker   *time.Ticker
	mapCount int
	value    []*pathMap
	curIndex int32
	request  chan string
	do       func(value []QPSInfo)
}

// QPSInfo 各个请求路径的qps归纳信息
type QPSInfo struct {
	Path  string
	Count int
}

type pathMap struct {
	sync.RWMutex
	qps map[string]int
}

var (
	once   sync.Once
	circle *circleTiming
)

// QPSTotal 统计全部请求的qps
func QPSTotal(interval time.Duration, doReport func(value []QPSInfo)) gin.HandlerFunc {
	once.Do(func() {
		circle = newDefaultCircle(interval, doReport)
		go circle.start()
	})

	return func(c *gin.Context) {
		key := c.Request.Method + "|" + c.Request.URL.Path
		circle.request <- key
		c.Next()
	}
}

func newDefaultCircle(interval time.Duration, doReport func(value []QPSInfo)) *circleTiming {
	paths := make([]*pathMap, 16)
	tic := time.NewTicker(interval)
	for i := range paths {
		paths[i] = &pathMap{
			qps: map[string]int{},
		}
	}
	return &circleTiming{
		ticker:   tic,
		mapCount: len(paths),
		value:    paths,
		do:       doReport,
		request:  make(chan string, 1000),
	}
}

func (p *circleTiming) start() {
	for {
		select {
		case <-p.ticker.C:
			oldIndex := atomic.LoadInt32(&p.curIndex)
			if oldIndex+1 == int32(p.mapCount) {
				atomic.StoreInt32(&p.curIndex, 0)
			} else {
				atomic.AddInt32(&p.curIndex, 1)
			}
			go p.report(int(oldIndex))
		case path := <-p.request:
			index := atomic.LoadInt32(&circle.curIndex)
			if _, ok := circle.value[index].qps[path]; !ok {
				circle.value[index].qps[path] = 0
			}
			circle.value[index].qps[path]++
		}
	}
}

func (p *circleTiming) report(index int) {
	// mapCount * ticker 时间内清理不完，可能会和start函数中产生冲突
	info := make([]QPSInfo, 0, len(p.value[index].qps))
	for key := range p.value[index].qps {
		info = append(info, QPSInfo{
			Path:  key,
			Count: p.value[index].qps[key],
		})
	}

	p.do(info)

	p.value[index].qps = map[string]int{}
}
