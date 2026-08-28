# minicloud


[![CI](https://github.com/aeliwat/mini-cloud-lab/actions/workflows/ci.yml/badge.svg)](https://github.com/aeliwat/mini-cloud-lab/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/aeliwat/mini-cloud-lab)](https://go.dev/)
[![License](https://img.shields.io/github/license/aeliwat/mini-cloud-lab)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/aeliwat/mini-cloud-lab.svg)](https://pkg.go.dev/github.com/aeliwat/mini-cloud-lab)


A local cloud simulator written in **Go**.

Provision virtual servers (web, database, load balancer), simulate traffic, and see what fails when capacity is exceeded — from the CLI or an animated UI.

---

## Build

```bash
git clone https://github.com/<your-username>/minicloud.git
cd minicloud
```

**Option A — local Go**

```bash
go build -o minicloud .
```

**Install the CLI globally** (so you can type `minicloud` from any folder):

```bash
# puts the binary in $(go env GOPATH)/bin  (usually ~/go/bin)
go install .

# make sure that folder is on your PATH (add to ~/.bashrc if needed)
export PATH="$(go env GOPATH)/bin:$PATH"
```

Then use it anywhere:

```bash
minicloud version
minicloud init
```

**Option B — Docker**

```bash
docker build -t minicloud .
```

Run CLI commands with Docker (state is saved in the current folder):

```bash
docker run --rm -v "$PWD":/work -w /work minicloud init
docker run --rm -v "$PWD":/work -w /work minicloud server add --type web --ram 2G --disk 50G
docker run --rm -v "$PWD":/work -w /work minicloud status
docker run --rm -v "$PWD":/work -w /work minicloud load --rps 1000 --duration 10s
```

Open the UI with Docker:

```bash
docker run --rm -p 7474:7474 -v "$PWD":/work -w /work minicloud ui --port 7474
# → http://localhost:7474
```

---

## Sample: first run

```bash
# start fresh (optional)
minicloud reset --force

minicloud init

minicloud server add --type lb  --ram 1G --disk 10G
minicloud server add --type web --ram 1G --disk 20G   # also creates a paired DB
minicloud server add --type web --ram 2G --disk 50G   # also creates a paired DB

minicloud status
```

Adding a **web** server provisions a dedicated database (`4G` / `100G`) and links them. Traffic path: users → LB → web-1 (then forward) → each web’s own DB.

---

## Virtual users & RPS

**Virtual users** are a label for the report — **RPS** is what actually stresses the system.

| Goal | Users | RPS |
|------|------:|----:|
| Light / healthy | 10,000 | 200–500 |
| Normal load | 50,000–100,000 | 1,000–2,000 |
| Stress / overload | 200,000–500,000 | 3,000–8,000 |

**Capacity (teaching model)**

| Role | Formula |
|------|---------|
| web | `RAM_GB × 500` RPS |
| lb | `RAM_GB × 2000` RPS |
| db | `RAM_GB × 1000` RPS |

Tip: start with `users=50000 / rps=1000 / 10s`, then raise RPS until nodes turn red. Or use UI scenarios (Black Friday, AZ failure, Tiny LB).

---

## Sample setups

### A — balanced (good first test)

```bash
minicloud reset --force && minicloud init

minicloud server add --type lb  --ram 1G --disk 10G
minicloud server add --type web --ram 2G --disk 40G   # + paired db 4G
minicloud server add --type web --ram 2G --disk 40G

# Capacities: LB 2000 · web 1000 each · db 4000 each
minicloud load --users 80000 --rps 1500 --duration 15s
```

Expect mostly green; web-1 near full, spillover to web-2.

### B — overload web tier

```bash
minicloud reset --force && minicloud init

minicloud server add --type lb  --ram 1G --disk 10G
minicloud server add --type web --ram 1G --disk 20G   # 500 rps each
minicloud server add --type web --ram 1G --disk 20G
minicloud server add --type web --ram 1G --disk 20G

minicloud load --users 300000 --rps 4000 --duration 20s
```

Fleet web cap ≈ 1500 RPS → failures / FULL on webs.

### C — tiny LB bottleneck

```bash
minicloud reset --force && minicloud init

minicloud server add --type lb  --ram 512M --disk 5G    # ~1000 rps
minicloud server add --type web --ram 4G   --disk 50G   # 2000 rps each
minicloud server add --type web --ram 4G   --disk 50G

minicloud load --users 200000 --rps 3000 --duration 15s
```

LB saturates first; webs stay healthier.

---

## Sample: simulate load (CLI)

```bash
minicloud load --rps 2000 --duration 10s --users 100000
```

**Example output:**
```text
Via LB:         true
Target RPS:     2000
Succeeded:      …
Failed:         …
Success rate:   …
```

---

## Sample: open the UI

```bash
minicloud ui
# open http://localhost:7474
```

In the UI you can:
- watch live second-by-second traffic
- drag nodes on the canvas
- run presets (Black Friday, AZ failure, Tiny LB)

---

## Usage

| Command | Description |
|---------|-------------|
| `minicloud init` | Create empty `state.json` |
| `minicloud reset --force` | Delete `state.json` and start over |
| `minicloud server add --type web\|db\|lb --ram 2G --disk 50G` | Add a server (`web` also creates a paired DB) |
| `minicloud server ls` | List servers |
| `minicloud server stop <id>` | Stop a server (LB will skip it) |
| `minicloud server start <id>` | Start a server and mark healthy |
| `minicloud server rm <id>` | Delete a server (web also deletes its paired DB) |
| `minicloud status` | Show fleet status + health |
| `minicloud load --rps 1000 --duration 15s [--users N]` | Simulate traffic (CLI) |
| `minicloud ui [--port 7474]` | Open animated UI |
| `minicloud version` | Print CLI version |
| `minicloud --help` | Show all commands |

**Server types**

| `--type` | Meaning | Capacity |
|----------|---------|----------|
| `web` | Web / API server (auto-pairs a DB) | `RAM × 500` RPS |
| `db` | Database | `RAM × 1000` RPS |
| `lb` | Load balancer | `RAM × 2000` RPS |

**Routing**
- With an LB: users → LB → **web-1 first**, then forward leftover RPS to web-2, …
- Each web sends traffic to **its own paired DB**

---

## Reset

If you see `minicloud is already initialized`:

```bash
minicloud reset --force
minicloud init
```
