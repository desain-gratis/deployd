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
wget https://github.com/desain-gratis/deployd/releases/latest/download/deployd-linux-amd64.tar.gz
```

---

## 2. Create Directories

```bash
sudo mkdir -p /opt/deployd
sudo mkdir -p /etc/deployd
```

---

## 3. Extract the Archive

Extract the release into the deployment directory.

```bash
sudo tar -xzf deployd-linux-amd64.tar.gz -C /opt/deployd
```

The resulting layout should look like:

```text
/opt/deployd/
└── deployd
```

Make the executable runnable:

```bash
sudo chmod +x /opt/deployd/deployd
```

---

## 4. Create the Config & Environment File


```bash
sudo mkdir -p /etc/deployd

sudo tee /etc/deployd/overwrite.env >/dev/null <<'EOF'
CONFIG=/etc/deployd/config.yaml
SECRET=/etc/deployd/secret.yaml
DEPLOYD_RAFT=/etc/deployd/raft.yaml
EOF

```


```bash 
sudo tee /etc/deployd/config.yaml >/dev/null <<'EOF'
host:
  id: <integer>
  name: <hostname>
  os: <os, eg. linux>
  architecture: <arch, eg. amd64>
  internal_address: deployd1 # can use ip

# base raft configuration for deployed apps
raft:
  replica_id: <integer, can be the same as host id>
  base_node_host_dir: "/data"
  base_wal_dir: "/data"

http:
  public:
    address: <http bind address eg. :9401>
    fqdn: <user accessible URL, eg. https://deployd.com>

ui:
  dir: "/var/www"

storage:
  s3:
    blob:
      endpoint: <s3 endpoint>
      key_id: <secret>
      key_secret: <secret>
      use_ssl: false
      bucket_name: <s3 bucket name>
      base_public_url: <public accessible URL of the bucket>
  file:
    config-data: "/data/deployd/config.db"
    job-data: "/data/deployd/job.db"
EOF

```

## 5. Create the Secret File

They have the same structure as env, and will overwrite the overwrite.env

```bash
sudo tee /etc/deployd/secret.yaml >/dev/null <<'EOF'

EOF
```



## 6. Create the systemd Service

```bash
sudo tee /etc/systemd/system/deployd.service >/dev/null <<'EOF'
[Unit]
Description=Deployd
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/deployd
ExecStart=/opt/deployd/deployd
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

```

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
