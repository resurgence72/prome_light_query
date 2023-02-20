package analysis

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"prome_query/pkg/cmd"
	"prome_query/pkg/redis"
	"strings"
	"sync"
	"time"
)

type record struct {
	Groups []*group `yaml:"groups"`
}

type group struct {
	Name  string  `yaml:"name"`
	Rules []*rule `yaml:"rules"`
}

type rule struct {
	Record string `yaml:"record"`
	Expr   string `yaml:"expr"`
}

// queryTimings with all query timers mapped to durations.
type queryTimings struct {
	EvalTotalTime        float64 `json:"evalTotalTime"`
	ResultSortTime       float64 `json:"resultSortTime"`
	QueryPreparationTime float64 `json:"queryPreparationTime"`
	InnerEvalTime        float64 `json:"innerEvalTime"`
	ExecQueueTime        float64 `json:"execQueueTime"`
	ExecTotalTime        float64 `json:"execTotalTime"`
}

// BuiltinStats holds the statistics that Prometheus's core gathers.
type builtinStats struct {
	Timings queryTimings `json:"timings,omitempty"`
}

type httpRequest struct {
	Path string `json:"path,omitempty"`
}

type params struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
	Query string `json:"query,omitempty"`
	Step  int    `json:"step,omitempty"`
}

type promQueryLog struct {
	HttpRequest httpRequest  `json:"httpRequest"`
	Params      params       `json:"params"`
	Stats       builtinStats `json:"stats"`
}

func StartAnalysis(dir string, prefix string) error {
	var (
		files []string
		wg    sync.WaitGroup
	)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if strings.HasPrefix(path, prefix) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	wg.Add(len(files))
	for _, file := range files {
		go analysis(file, &wg)
	}

	wg.Wait()

	// 读取redis 生成record yaml 分发给prome
	dispatch()
	return nil
}

func dispatch() {
	tgs := redis.HGetAllByKey("PROMETHEUS:HCS")
	if len(tgs) == 0 {
		return
	}

	var rules []*rule
	for old, new := range tgs {
		rules = append(rules, &rule{
			Record: new,
			Expr:   old,
		})
	}

	g := &record{Groups: []*group{
		{
			Name:  "auto-heavy-query-replace",
			Rules: rules,
		},
	}}

	data, _ := yaml.Marshal(g)
	ioutil.WriteFile("./record.yml", data, 0640)

	err := cmd.Cmd.Push("./record.yml", "/rules/record.yml")
	if err != nil {
		fmt.Println("push err ", err)
	}
}

func analysis(path string, wg *sync.WaitGroup) {
	defer wg.Done()

	bs, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("readfile err")
		return
	}

	var (
		blocks []string
		hcs    []string
	)

	for _, line := range bytes.Fields(bs) {
		log := new(promQueryLog)
		if err = json.Unmarshal(line, log); err != nil {
			fmt.Println("json unmarshal err", err)
			continue
		}

		if checkIsBlock(log) {
			blocks = append(blocks, log.Params.Query)
			continue
		}

		if checkIsHighCardinality(log) {
			hcs = append(hcs, log.Params.Query)
		}

	}

	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
	redis.Rdb.SAdd(ctx, "PROMETHEUS:BLOCKS", blocks)
	redis.Rdb.Expire(ctx, "PROMETHEUS:BLOCKS", 2*24*time.Hour)

	batchSet("PROMETHEUS:HCS", hcs)
}

func getMD5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func getRecordName(v string) string {
	key := "PHQ:" + getMD5(v)
	return key
}

func batchSet(key string, vs []string) {
	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
	old := redis.HGetAllByKey(key)

	delm := make(map[string]string)
	newm := make(map[string]struct{})
	for _, v := range vs {
		newm[v] = struct{}{}
	}

	for k := range old {
		if _, ok := newm[k]; !ok {
			delm[k] = old[k]
		}
	}

	if len(delm) > 0 {
		var dels []string
		for k, v := range delm {
			dels = append(dels, k, v)
		}

		redis.Rdb.HDel(ctx, key, dels...)
	}

	m := make(map[string]string, len(vs))
	for _, v := range vs {
		//redis.Rdb.HSet(ctx, key, v, getRecordName(v))
		m[v] = getRecordName(v)
	}
	redis.Rdb.HMSet(ctx, key, m)
	redis.Rdb.Expire(ctx, key, 2*24*time.Hour)
}

func checkIsBlock(log *promQueryLog) bool {
	blocks := []string{
		`{__name__=~".*"}`,
		`count({__name__=~".*"})`,
		`sum({__name__=~".*"})`,
		`{__name__=~".*."}`,
		`count({__name__=~".*."})`,
		`sum({__name__=~".*."})`,
		`{__name__=~".+"}`,
		`count({__name__=~".+"})`,
		`sum({__name__=~".+"})`,
		`{__name__=~".+."}`,
		`count({__name__=~".+."})`,
		`sum({__name__=~".+."})`,
	}

	for _, block := range blocks {
		if log.Params.Query == block {
			return true
		}
	}
	return false
}

func checkIsHighCardinality(log *promQueryLog) bool {
	// 判断 QueryPreparationTime 时间
	qpt := log.Stats.Timings.QueryPreparationTime

	// 0.00005 s
	t := 1000 * time.Microsecond

	if t.Seconds() < qpt {
		fmt.Println("时间北卡 ", t.Seconds(), qpt)
		return false
	}

	// 判断是否是range query
	if log.HttpRequest.Path != "/api/v1/query_range" {
		return false
	}

	// 判断查询范围是否小与3h
	s, _ := time.Parse("2006-01-02T15:04:05Z", log.Params.Start)
	e, _ := time.Parse("2006-01-02T15:04:05Z", log.Params.End)

	// 如果重查询时3h内的 则判定为重查询
	if e.Sub(s) > 3*time.Hour {
		return false
	}

	return true
}
