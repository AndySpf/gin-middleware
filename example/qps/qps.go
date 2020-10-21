package main

import (
	"fmt"
	"ginmidd/middleware"
	"github.com/gin-gonic/gin"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	h := gin.New()
	h.Use(middleware.QPSTotal(time.Second*2, doReportQps))
	h.GET("/test3", func(context *gin.Context) {
		context.JSON(200, map[string]string{"msg": "ok1"})
		return
	})
	h.GET("/test2", func(context *gin.Context) {
		context.JSON(200, map[string]string{"msg": "ok2"})
		return
	})
	h.GET("/test0", func(context *gin.Context) {
		context.JSON(200, map[string]string{"msg": "ok0"})
		return
	})
	go h.Run(":9998")
	for i := 0; i < 10000; i++ {
		go func(index int) {
			res, err := http.Get(fmt.Sprintf("http://127.0.0.1:9998/test%d", index%3))
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			io.Copy(ioutil.Discard, res.Body)
			res.Body.Close()
		}(i)
		time.Sleep(time.Microsecond * 100)
	}
	time.Sleep(time.Second * 30)
	fmt.Println(total)
}

var total = 0

func doReportQps(value []middleware.QPSInfo) {
	for i := range value {
		total += value[i].Count
	}
	fmt.Println(value)
}
