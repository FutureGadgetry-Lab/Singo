# Singo

一个专门查看 sing-box 服务端**按用户流量**的小面板。Singo 是单个 Go 二进制，HTML/CSS/JS 全部内嵌；运行时不需要 Node.js、数据库或 Docker，适合 512 MB 内存的小服务器。

![状态](https://img.shields.io/badge/runtime-single_binary-73e2a7)

## 能做什么

- 查看全部用户的上传、下载和累计流量
- 查看最近 14 天柱状图（状态文件默认保留 90 天）
- 自动发现产生过流量的用户
- JSON 文件原子持久化，异常退出最多损失一个采集周期
- 可选 HTTP Basic Auth；默认只监听本机

Singo 是观察面板，不修改 sing-box 配置，也不负责用户增删或限额。

## 1. 配置 sing-box

按用户统计需要 sing-box 的 V2Ray API。官方预编译包默认不含此功能，请确认：

```bash
sing-box version
```

输出的 `Tags` 必须包含 `with_v2ray_api`。然后在 sing-box 配置中加入（用户名称按实际修改）：

```json
{
  "experimental": {
    "v2ray_api": {
      "listen": "127.0.0.1:8080",
      "stats": {
        "enabled": true,
        "users": ["alice", "bob"]
      }
    }
  }
}
```

这里的名字必须和 inbound 中每个用户的 `name` 一致。V2Ray API 只监听 `127.0.0.1`，不要暴露到公网。修改后检查并重启：

```bash
sing-box check -c /etc/sing-box/config.json
sudo systemctl restart sing-box
```

> sing-box 重载/重启会清空它尚未被采集的内存计数器。Singo 默认每 10 秒采集一次，因此重载边界理论上可能少记不超过一个采集周期的流量；它适合观察，不建议直接用于精确计费。

## 2. 构建和直接运行

需要 Go 1.23 或更新版本，仅构建时需要依赖：

```bash
make build
./dist/singo
```

打开 `http://127.0.0.1:8088`。常用参数：

```text
-listen 127.0.0.1:8088      面板监听地址
-grpc 127.0.0.1:8080        sing-box V2Ray API 地址
-data ./singo-data.json      状态文件
-interval 10s                采集周期（最小 1s）
-keep-days 90                日历史保留天数
-user USER -password PASS    启用 Basic Auth
```

对应环境变量为 `SINGO_LISTEN`、`SINGO_GRPC`、`SINGO_DATA`、`SINGO_INTERVAL`、`SINGO_KEEP_DAYS`、`SINGO_USER`、`SINGO_PASSWORD`，命令行参数优先。

## 3. systemd 安装

```bash
sudo install -m 0755 dist/singo /usr/local/bin/singo
sudo useradd --system --home /var/lib/singo --shell /usr/sbin/nologin singo
sudo install -d -o singo -g singo -m 0750 /var/lib/singo
sudo install -m 0644 singo.service /etc/systemd/system/singo.service
sudo systemctl daemon-reload
sudo systemctl enable --now singo
sudo systemctl status singo
```

如果要从公网访问，建议保持 Singo 监听本机，用 Caddy/Nginx 反向代理并启用 HTTPS。也可以给 service 增加环境变量：

```ini
Environment=SINGO_USER=admin
Environment=SINGO_PASSWORD=请换成长随机密码
```

修改 unit 后执行 `sudo systemctl daemon-reload && sudo systemctl restart singo`。

## 资源占用设计

Singo 只有一个进程和一个很小的 JSON 文件；前端无框架，服务端没有 SQLite/CGO。systemd 示例将内存上限设为 64 MB。用户数和历史天数非常大时 JSON 会线性增长，可以调低 `-keep-days`。
