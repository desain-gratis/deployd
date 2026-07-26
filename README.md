# Deployd

deploy golang service as ubuntu systemd service

# Installation

This guide describes how to manually install **deployd** as a systemd service.

## Requirements

* Linux x86_64 (amd64)
* systemd
* Root or sudo access

## Directory Layout

Deployd is installed under:

```text
/opt/deployd/current/
```

Environment files are stored in:

```text
/etc/deployd/env/
```

The service definition is:

```text
/etc/systemd/system/deployd.service
```

---

## 1. Download the Release

Download the latest release from GitHub.

```bash
wget https://github.com/<owner>/<repo>/releases/latest/download/deployd-linux-amd64.tar.gz
```

---

## 2. Create Directories

```bash
sudo mkdir -p /opt/deployd/current
sudo mkdir -p /etc/deployd/env
```

---

## 3. Extract the Archive

Extract the release into the deployment directory.

```bash
sudo tar -xzf deployd-linux-amd64.tar.gz -C /opt/deployd/current
```

The resulting layout should look like:

```text
/opt/deployd/current/
└── deployd
```

Make the executable runnable:

```bash
sudo chmod +x /opt/deployd/current/deployd
```

---

## 4. Create Environment File

Create:

```text
/etc/deployd/env/overwrite.env
```

Example:

```bash
# Example configuration
PORT=8080

# Add additional configuration here
```

The environment file is optional. Missing files are ignored.

---

## 5. Create the systemd Service

Create:

```text
/etc/systemd/system/deployd.service
```

Contents:

```ini
[Unit]
Description=Deployd
After=network.target

[Service]
Type=simple
EnvironmentFile=-/etc/deployd/env/overwrite.env
WorkingDirectory=/opt/deployd/current/
ExecStart=/opt/deployd/current/deployd
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

---

## 6. Enable the Service

Reload systemd:

```bash
sudo systemctl daemon-reload
```

Enable deployd to start automatically:

```bash
sudo systemctl enable deployd
```

Start it:

```bash
sudo systemctl start deployd
```

---

## 7. Verify Installation

Check service status:

```bash
sudo systemctl status deployd
```

Follow logs:

```bash
sudo journalctl -u deployd -f
```

---

## Updating Deployd

1. Stop the service.

```bash
sudo systemctl stop deployd
```

2. Replace the binary.

```bash
sudo tar -xzf deployd-linux-amd64.tar.gz -C /opt/deployd/current
sudo chmod +x /opt/deployd/current/deployd
```

3. Restart.

```bash
sudo systemctl restart deployd
```

---

## Directory Summary

```text
/opt/deployd/current/
    deployd

/etc/deployd/
    env/
        overwrite.env

/etc/systemd/system/
    deployd.service
```
