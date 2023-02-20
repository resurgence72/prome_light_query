package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httputil"
	"net/url"
	"prome_query/pkg/analysis"
	"prome_query/pkg/cmd"
	"prome_query/pkg/redis"
	"time"
)

func main() {
	if err := redis.NewRedis(); err != nil {
		panic(err)
	}
	//ana()
	gw()
}

func ana() {

	localLogPrefix := "p8s.log"

	c := cmd.NewCmdRunner(
		"root",
		"1",
		[]string{"10.0.0.105"},
		time.Second*5,
	)
	err := c.Pull("/root/prometheus-2.41.0.linux-amd64/log/prometheus.log", "./"+localLogPrefix)
	if err != nil {
		panic(err)
	}

	analysis.StartAnalysis("./", localLogPrefix)
}

type queryParams struct {
	Query string `form:"query" json:"query"`
	Start int64  `form:"start" json:"start"`
	End   int64  `form:"end" json:"end"`
	Step  int64  `form:"step" json:"step"`
}

func gw() {
	r := gin.Default()
	target := "http://10.0.0.105:9090"
	proxyUrl, _ := url.Parse(target)

	r.Any("/*name", func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != "/api/v1/query_range" {
			c.Request.URL.Path = c.Param("name") //  重点是这行代码
		} else {
			fmt.Println("替换前 ",c.Request.URL.RawQuery)
			m := redis.HGetAllByKey("PROMETHEUS:HCS")
			switch c.Request.Method {
			case "GET":
				q := new(queryParams)
				c.ShouldBindQuery(q)
				if rep, ok := m[q.Query]; ok {
					q.Query = rep
				}

				step := time.Duration(q.Step) * time.Second
				c.Request.URL.RawQuery = fmt.Sprintf(
					"query=%s&start=%s&end=%s&step=%s",
					q.Query,
					changeTime(q.Start),
					changeTime(q.End),
					step.String(),
				)
				fmt.Println("替换后 ",c.Request.URL.RawQuery)
			case "POST":
			}
		}
		proxy := httputil.NewSingleHostReverseProxy(proxyUrl)
		proxy.ServeHTTP(c.Writer, c.Request)
	})
	r.Run(":9090")
}


func changeTime(t int64) string {
	res := time.Unix(t,0).UTC().Format("2006-01-02T15:04:05Z")
	return res
}