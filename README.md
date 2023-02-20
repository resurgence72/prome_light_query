### prome_light_query

> 每天定时拉取 prometheus log，分析出 heavy_query promQL, 基于 Redis 动态生成 record；
>
> 自身作为 gw 原生适配 grafana 查询请求并做 heavy_query 动态替换