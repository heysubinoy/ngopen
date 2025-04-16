# npopen

`npopen` is a lightweight, Go-based reverse tunneling and registry server. It enables developers to securely expose local services to the internet using ephemeral hostnames and access tokens—without punching holes in firewalls or forwarding ports.

Think of it like a minimalistic, self-hostable version of `ngrok`, built with transparency and performance in mind.

---

## ✨ Features

- 🔐 **Token-based access** – Secure connections with pluggable validation
- 🌐 **Ephemeral hostname generation** – Unique identifiers per tunnel
- 📦 **Service registry** – Lightweight in-memory tracking for active tunnels
- 🧩 **Stream framing protocol** – Reliable raw TCP stream handling
- 🧵 **Concurrent stream parsing** – Efficient routing & control handling

---

## 📂 Project Structure

```text
.
├── framing.go             # TCP stream framing/deframing logic
├── go.mod                 # Module definition
├── go.sum                 # Dependency checksums
├── hostname-generator.go # Random hostname generation logic
├── main.go                # Main entrypoint; server logic
├── parser.go              # Incoming stream parser
├── registry.go            # Registry for managing tunnel endpoints
└── token-validator.go     # Token validation logic
```

---

## 🚀 Getting Started

### 🧾 Prerequisites

- Go 1.20 or later

### ⚙️ Installation

```bash
git clone https://github.com/heysubinoy/npopen.git
cd npopen/server
go build -o npopen
```

### 🔧 Usage

Start the server:

```bash
./npopen
```

By default, the server listens on port `80`.

---

## 🧠 How It Works

1. A client connects and provides an access token.
2. Token is validated via logic in `token-validator.go`.
3. A hostname is generated (`hostname-generator.go`) and registered (`registry.go`).
4. Data is framed (`framing.go`), parsed (`parser.go`), and routed accordingly.

This makes it possible to expose a local service like `localhost:3000` over the internet with zero config.

---

## 🔒 Token Validation

Custom token logic is implemented in `token-validator.go`.

You can adapt it for:
- JWT-based auth
- API keys
- Temporary session tokens
- Or even OAuth if you’re feeling fancy

---

## 🌐 Hostname Generator

Hostnames are generated dynamically—think `cool-weasel-3941.npopen.dev`. This avoids collisions and helps identify tunnel connections.

Modify `hostname-generator.go` to customize the format or use vanity names.

---

## 🛠 Configuration

You can tweak:
- Listening port (in `main.go`)
- Token strategy (`token-validator.go`)
- Hostname format (`hostname-generator.go`)
- Persistence layer for registry (`registry.go`)

Future versions will include a config file or ENV support.

---

## 🧪 Testing

Currently under development. For now, testing is manual via CLI clients. Formal test suite coming soon™.

---

## 📦 Planned

- [x] TLS support (Let's Encrypt or cert mount)
- [ ] Web dashboard for monitoring
- [x] Client CLI for easy tunnel creation
- [ ] Support for HTTP/WebSocket tunnels
- [ ] Docker deployment support

---

## 🤝 Contributing

Contributions are very welcome!

Fork the repo → create a branch → submit a PR.  
Bug reports, feature suggestions, and memes are appreciated.

---

## 👋 Author

Made with ☕ and frustration by [Subinoy Biswas](https://subinoy.me)

---

> npopen: Not Production-ready. Probably Open. Definitely Nerdy.
