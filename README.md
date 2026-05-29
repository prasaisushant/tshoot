# 🚀 tshoot

A lightweight, zero-dependency terminal user interface (TUI) designed for instantaneous Linux server diagnostics and infrastructure troubleshooting. Built natively in Go.

![tshoot Dashboard Overview](https://github.com/prasaisushant/tshoot/raw/main/assets/tshoot_dashboard.png)

---

## 📌 What is `tshoot`?

The first step of debugging any infrastructure issue is almost always identical:
1. **Check core resource strains** (Is the CPU or Memory pinned?)
2. **Audit storage boundaries** (Are any critical disks or mount partitions at 100%?)
3. **Verify active communication lines** (What networking ports are actively listening?)
4. **Test network egress and routing** (Are core DNS lookups and external pings dropping packets?)
5. **Inspect application layers** (Are the Docker containers running smoothly, or are their logs throwing exceptions?)
6. **Isolate resource hogs** (Which exact Process IDs are consuming the host’s overhead?)

Instead of manually executing an exhausting sequence of isolated commands like `top`, `df -h`, `ss -tulpn`, `ping`, and `docker logs` one by one, **`tshoot` integrates all of these essential first-step diagnostic vectors into a singular, real-time dashboard.**

> ⚠️ **Scope Definition:** `tshoot` is explicitly not built to be a bloated, all-in-one performance suite or an all-encompassing tracing tool. It is engineered to do one thing flawlessly: accelerate the **initial discovery phase** of your incident response pipeline so you can pinpoint anomalies in seconds.

---

## 🧠 Why It Was Created

`tshoot` fills the gap as your first line of defense during infrastructure anomalies, focusing entirely on speed, accuracy, and survivability:
* **Direct Kernel Communication:** It does not scrape output from shell utilities or look for system binaries. It parses data directly from the Linux kernel's `/proc` filesystem.
* **Low Footprint:** Compiled as a clean, highly optimized Go binary, it runs safely on strangled servers without worsening resource exhaustion.
* **100% Portable:** A single statically linked binary with absolute zero dependencies. It executes perfectly on any Linux environment without requiring a Go installation, runtime interpreters, or system library configurations.

---

## ✨ Features & Navigation

The interface splits your terminal space into high-density, interactive panels that can be expanded, filtered, or configured via keyboard inputs.

### Core Modules
* **CPU / Memory / System:** Live computation tracking using delta jiffies calculation from `/proc/stat` and memory breakdown matrixes from `/proc/meminfo`.
* **Dynamic Storage Panel:** Tracks live disk partition mappings, storage filesystems, and usage scales.
* **Network & Ping Engine:** Monitors persistent outbound packet status, connectivity status, and microsecond latencies.
* **Process Priority Map:** Tracks specific resource consumers, organizing active system threads dynamically by peak usage thresholds.
* **Port Mapping:** Cross-references open sockets and tracks down exactly which process IDs (PIDs) own specific ports.
* **Container Layer:** Connects directly with the Docker daemon API to monitor runtime container statuses and inspect system log outputs.

### Keyboard Shortcuts
| Key | Action |
| :--- | :--- |
| `F1` | **Refresh Settings:** Opens a modal to adjust the interface update intervals (`1s`, `3s`, `5s`, `10s`). |
| `F2` | **Docker Controls:** Opens a detailed modal list to select containers and view their logs. |
| `F3` | **Ping Directory:** Opens target management to monitor infrastructure endpoints populated by a local configuration file. |
| `F4` | **Maximize Mode:** Instantly scales the currently highlighted panel into full screen to inspect additional text blocks. |
| `s` | **Storage Toggle:** Dynamically cycles storage panel printouts between disk mounts and raw block device topologies. |
| `Esc` / `b` | Close active modals, clear filter streams, or leave maximized views. |
| `q` | Immediately exit the application. |

---

## 💾 Installation

You can securely bootstrap `tshoot` onto any Linux computer using our unified, single-line installation pipeline. The script automatically manages the latest release targeting, downloads the architecture-specific asset, adjusts permissions, and routes it to your global application paths.

```bash
curl -sSL https://raw.githubusercontent.com/prasaisushant/tshoot/main/install.sh | bash