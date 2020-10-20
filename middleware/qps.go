package middleware

import (
	"github.com/gin-gonic/gin"
	"sync"
	"sync/atomic"
	"time"
)

type circleTiming struct {
	ticker   *time.Ticker
	interval time.Duration
	period   time.Duration
	value    []*pathMap
	curIndex int32
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
func QPSTotal(doReport func(value []QPSInfo)) gin.HandlerFunc {
	once.Do(func() {
		circle = newDefaultCircle(doReport)
		go circle.start()
	})

	return func(c *gin.Context) {
		key := c.Request.Method + "|" + c.Request.URL.Path
		index := atomic.LoadInt32(&circle.curIndex)
		circle.value[index].Lock()
		if _, ok := circle.value[index].qps[key]; ok {
			circle.value[index].qps[key] = 0
		}
		circle.value[index].qps[key]++
		circle.value[index].Unlock()

		c.Next()
	}
}

func newDefaultCircle(doReport func(value []QPSInfo)) *circleTiming {
	paths := make([]*pathMap, 60)
	tic := time.NewTicker(time.Second)
	for i := range paths {
		paths[i] = &pathMap{
			qps: map[string]int{},
		}
	}
	return &circleTiming{
		ticker:   tic,
		interval: time.Second,
		period:   time.Minute,
		value:    paths,
		do:       doReport,
	}
}

func (p *circleTiming) start() {
	select {
	case <-p.ticker.C:
		go p.report()
		if p.curIndex+1 == int32(p.period/p.interval) {
			atomic.StoreInt32(&p.curIndex, 0)
		} else {
			atomic.AddInt32(&p.curIndex, 1)
		}
	}
}

func (p *pathMap) resetQPSCount() {
	p.Lock()
	for key := range p.qps {
		p.qps[key] = 0
	}
	p.Unlock()
}

func (p *circleTiming) report() {
	index := atomic.LoadInt32(&p.curIndex)
	p.value[index].RLock()
	info := make([]QPSInfo, 0, len(p.value[index].qps))
	for key := range p.value[index].qps {
		info = append(info, QPSInfo{
			Path:  key,
			Count: p.value[index].qps[key],
		})
	}
	p.value[index].RUnlock()

	p.do(info)

	p.value[p.curIndex].resetQPSCount()
}
