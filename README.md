# minicloud

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
minicloud server add --type web --ram 1G --disk 20G
minicloud server add --type web --ram 2G --disk 50G
minicloud server add --type db  --ram 4G --disk 100G

minicloud status
```

**Example output:**
```text
State:    .../state.json
Servers:  4

ID      ROLE  RAM  DISK  STATUS
lb-…    lb    1G   10G   running
web-…   web   1G   20G   running
web-…   web   2G   50G   running
db-…    db    4G   100G  running
```

---

## Sample: simulate load (CLI)

```bash
./minicloud load --rps 2000 --duration 10s --users 100000
```

**Example output:**
```text
Via LB:         true
Target RPS:     2000
Succeeded:      …
Failed:         …
Success rate:   …
```



## Sample: open the UI

```bash
./minicloud ui
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
| `minicloud server add --type web\|db\|lb --ram 2G --disk 50G` | Add a server |
| `minicloud server ls` | List servers |
| `minicloud server stop <id>` | Stop a server (LB skips it) |
| `minicloud server start <id>` | Start a server and mark healthy |
| `minicloud server rm <id>` | Delete a server |
| `minicloud status` | Show fleet status + health |
| `minicloud load --rps 1000 --duration 15s [--users N]` | Simulate traffic (CLI) |
| `minicloud ui [--port 7474]` | Open animated UI |
| `minicloud version` | Print CLI version |
| `minicloud --help` | Show all commands |

**Server types**

| `--type` | Meaning | Capacity |
|----------|---------|----------|
| `web` | Web / API server | `RAM × 500` RPS |
| `db` | Database | shown in UI only |
| `lb` | Load balancer | `RAM × 2000` RPS |

**Health & latency**
- Sustained overload (~3s) marks a web node **unhealthy**; the LB stops routing to it
- `server start <id>` brings it back healthy
- Load reports include **avg / p50 / p95** latency (ms)

---

## Reset

If you see `minicloud is already initialized`:

```bash
minicloud reset --force
minicloud init
```
