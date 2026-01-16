# limiter

*Run it. But carefully.*

---

A simple wrapper around `systemd-run` for resource-limited execution.

## Setup

**One-time system configuration** (requires root):

Resource control requires delegating systemd capabilities to user sessions.

```sh
# Create the directory
sudo mkdir -p /etc/systemd/system/user@.service.d

# Write the delegation configuration
sudo tee /etc/systemd/system/user@.service.d/delegate.conf > /dev/null <<EOF
[Service]
Delegate=cpu cpuset memory pids
EOF

# Apply changes
sudo systemctl daemon-reload
```

TODO: add the setup script

## Use

```sh
limiter -ram 2G -cores 1.5 -timeout 1h -- your-command
```

## Build

```sh
go build
```

## Why

Setting boundaries lets you sleep at peace.

In a shared environment, it's important to limit the impact of runaway processes. Limiter provides a simple way to do this with little cognitive load.


---

**License**: GPL-3
