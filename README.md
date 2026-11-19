# ngopen

A self-hosted reverse tunnel, written in Go. Run `ngopen` next to a service on your laptop and it gets a public HTTPS hostname, without opening ports or touching your firewall. Think of it as a small, readable ngrok you can run on your own server.

```text
$ ngopen --local localhost:3000 --auth <token>
✓ Tunnel established
✓ Forwarding https://brave-otter-4821.n.sbn.lol -> localhost:3000
✓ Ready for connections
```

## How it works

```text
                        public internet                         your machine
 ┌─────────┐  HTTPS   ┌──────────────┐  HTTP   ┌──────────────┐   one TCP conn,   ┌──────────────┐  HTTP  ┌───────────────┐
 │ browser │ ───────▶ │ reverse proxy│ ──────▶ │ ngopen server│ ◀════════════════ │ ngopen client│ ─────▶ │ localhost:3000│
 └─────────┘          │  (TLS, *.dom)│  :8080  │              │  many smux streams│              │        └───────────────┘
                      └──────────────┘         └──────────────┘       :9000       └──────────────┘
```

1. **Dial out, not in.** The client opens a single outbound TCP connection to the server on `:9000`. Because the connection starts from inside your network, no inbound port needs to be open.
2. **Multiplex.** That connection is wrapped in an [smux](https://github.com/xtaci/smux) session. Every proxied HTTP request gets its own lightweight stream, so many requests share one connection without blocking each other.
3. **Authenticate on stream 0.** The first stream carries the auth handshake: the client sends its token and requested hostname, and the server validates the token against an external auth service and assigns a hostname such as `brave-otter-4821.n.sbn.lol`.
4. **Route by Host header.** The server keeps an in-memory registry of `hostname → session`. When a request arrives on `:8080`, it looks up the `Host` header, opens a new stream to the matching client, and forwards the request.
5. **Replay locally.** The client reads the request off the stream, replays it against your local service, and writes the response back on the same stream.

If the connection drops, the client retries automatically after a configurable delay.

## Wire protocol

Every message is a length-prefixed frame:

```text
┌────────────────────┬──────────────────────────┐
│ length (uint32 BE) │ payload (length bytes)    │
└────────────────────┴──────────────────────────┘
```

| Stream | Direction | Payload |
|---|---|---|
| auth (first stream) | client → server | `AUTHTOKEN:<token>\nHOSTNAME:<name or AUTO>\n` |
| auth (first stream) | server → client | `OK:<assigned-hostname>` or `FAIL:<reason>` |
| one per request | server → client | raw HTTP/1.1 request (`http.Request.Write`) |
| one per request | client → server | raw HTTP/1.1 response |

Because the auth frame arrives before the client is trusted, its declared length is capped at 4 KiB. A client claiming a larger frame is rejected before any memory is allocated for it.

## Quick start

Requires Go 1.22+.

```bash
git clone https://github.com/heysubinoy/ngopen.git
cd ngopen
go build -o ngopen .
```

### Server

```bash
export API_VALIDATE_URL=https://auth.example.com/validate   # token validation endpoint
export NGOPEN_HOSTNAME_SUFFIX=.tunnel.example.com          # wildcard domain pointed at this box
./ngopen server
```

The server listens on `:9000` for tunnel clients and `:8080` for public HTTP traffic. In production, put a reverse proxy such as Caddy or nginx in front of `:8080` to terminate TLS for `*.tunnel.example.com`.

### Client

```bash
./ngopen --local localhost:3000 --auth <token> --server tunnel.example.com:9000
```

Save your settings so you don't have to pass them every time:

```bash
./ngopen config set auth-key <token>
./ngopen config set server tunnel.example.com:9000
./ngopen config list
```

## Configuration

**Server (environment variables)**, see [`.env.example`](.env.example):

| Variable | Purpose | Default |
|---|---|---|
| `API_VALIDATE_URL` | Endpoint the server POSTs `{"key": "<token>"}` to. It must reply `200` with `{"valid": true}`. | required |
| `NGOPEN_HOSTNAME_SUFFIX` | Domain suffix for generated hostnames | `.n.sbn.lol` |
| `NGOPEN_LOG_LEVEL` | `DEBUG`, `INFO`, `WARN`, `ERROR` | `INFO` |
| `NGOPEN_MODE` | `DEV` for local development | prod |

**Client (flags, or `NGOPEN_*` environment variables)**:

| Flag | Purpose | Default |
|---|---|---|
| `--local` | Local service to expose | required |
| `--auth` | Auth token | required |
| `--server` | Tunnel server address | `connect.n.sbn.lol:9000` |
| `--hostname` | Hostname to request, or `AUTO` | `AUTO` |
| `--reconnect-delay` | Wait between reconnect attempts | `5s` |
| `--preserve-ip` | Pass the visitor's IP through in `X-Forwarded-For` | `true` |
| `--debug` | Verbose logs | `false` |

## Design notes

- **Fails closed on auth.** Token validation treats an unset URL, a network error, a timeout (5 s), a non-200 response, or an unreadable body as *invalid*. Tokens are never written to logs.
- **Per-request deadlines.** Each proxied request has a 1-minute read/write deadline on its stream, so one slow local handler can't hold a stream open forever.
- **Per-request errors stay per-request.** A failed request returns `502` to that caller only. The client's session is torn down only when the stream itself can't be opened.

## Testing

```bash
go test -race ./protocol/ ./server/
```

The tests cover the auth frame round trip and the oversized-frame and truncated-frame errors, a full request/response round trip through the tunnel framing over an in-memory connection, hostname generation, and token validation against a fake auth service.

## Limitations and roadmap

- [ ] TLS on the client ↔ server tunnel link. Public traffic is HTTPS via the reverse proxy, but the `:9000` link is currently plain TCP.
- [ ] Stream request and response bodies instead of buffering them whole
- [ ] Hostname reclaim on reconnect, and reserved custom hostnames, tied to the auth token. Currently the server only accepts `AUTO`.
- [ ] WebSocket and raw TCP tunnels
- [ ] Metrics and a small dashboard

## Author

Built by [Subinoy Biswas](https://subinoy.me). There's a longer write-up on [Medium](https://heysubinoy.medium.com).
