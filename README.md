# limiter

*Run it. But carefully.*

---

A simple wrapper around `systemd-run` for resource-limited execution on Linux.

## Setup

**One-time system configuration** (requires root):

```sh
sudo limiter setup
```

This creates:
- Systemd delegation config for resource control
- Default limits at `/etc/limiter/limits.cfg`

Verify it worked:

```sh
limiter verify
```

## Use

```sh
limiter -- your-command              # Uses defaults
limiter -ram 2G -- your-command      # Override specific limits
limiter -ram 2G -cores 1.5 -timeout 1h -- your-command
```

## Defaults

If not specified via flags, limits are read from `/etc/limiter/limits.cfg`:

```
ram=1G
cores=3.2      # 20% of system cores (auto-detected)
procs=100
timeout=7d
```

Built-in fallback:
- **ram**: 1G
- **cores**: 20% of available CPU cores
- **procs**: 100
- **timeout**: 7 days

CLI flags override config file, config file overrides built-in defaults.

## Build

```sh
go build
```

Or use the Makefile:

```sh
make build          # Build for current platform
make test           # Run tests with coverage
make install        # Install to /usr/local/bin (requires sudo)
make build-all      # Build for Linux (amd64/arm64)
```

## Why

Setting boundaries lets you sleep at peace.

In a shared environment, it's important to limit the impact of runaway processes. Limiter provides a simple way to do this.

---

**License**: GPL-3