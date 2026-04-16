# qq2rss

Convert OneBot 11 group link messages into a persistent RSS feed.

## Run

```bash
go run .
```

Optionally pass a config path:

```bash
go run . config.json
```

## Config

Edit [config.json](config.json):
- `listen_addr`: RSS/health endpoint (default `:8080`)
- `onebot_listen_addr`: OneBot webhook endpoint (default `:8081`)
- `feed.group_id`: only this group is accepted

## Endpoints

- `GET /rss` on `listen_addr`
- `POST /onebot` on `onebot_listen_addr`
- `GET /healthz` on `listen_addr`
