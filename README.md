# Deployd

deploy golang service as ubuntu systemd service

# Installation

This guide describes how to manually install **deployd** as a systemd service.

## Requirements

* Linux x86_64 (amd64)
* systemd
* Root or sudo access
* 3 hosts/PC in with private static internal IP

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
wget https://github.com/desain-gratis/deployd/releases/download/v0.0.2/deployd-linux-amd64.tar.gz
```

---

## 2. Extract the Archive

Extract the release into the deployment directory.

```bash
sudo mkdir -p /opt/deployd
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

## 3. Create the Config & Environment File


```bash
sudo mkdir -p /etc/deployd
sudo tee /etc/deployd/overwrite.env >/dev/null <<'EOF'
CONFIG=/etc/deployd/config.yaml
SECRET=/etc/deployd/secret.yaml
EOF

```

Next, create the config. Please adjust it according to your own host information.
For example, we have 3 hosts with these internal-local IP address. 

* host1 (100.0.0.1), ID: 1
* host2 (100.0.0.2), ID: 2
* host3 (100.0.0.3), ID: 3

```bash 
sudo tee /etc/deployd/config.yaml >/dev/null <<'EOF'
host:
  id: 1
  name: host1
  os: linux
  architecture: amd64
  internal_address: 100.0.0.1

# base raft configuration for deployed apps
raft:
  replica_id: 1
  base_node_host_dir: "/data"
  base_wal_dir: "/data"
  etcd_config: "/etc/deployd/etcd-raft.yaml"

http:
  public:
    address: 100.0.0.1:9401
    fqdn: http://host1.com:9401

ui:
  dir: "/var/www"

storage:
  s3:
    blob:
      endpoint: <s3 endpoint>
      key_id: <s3 access key id>
      key_secret: <s3 key secret>
      use_ssl: false
      bucket_name: <s3 bucket name>
      base_public_url: <public accessible URL of the bucket>
  file:
    config-data: "/data/deployd/config.db"
    job-data: "/data/deployd/job.db"
EOF

```

## 4. Create the Secret File

They can be used to overwrite config.yaml with secret. (but now it's not used)

```bash
sudo tee /etc/deployd/secret.yaml >/dev/null <<'EOF'

EOF
```

## 5. Create the Etcd Raft File

Etcd raft is what makes this a distributed application.

```bash
sudo tee /etc/deployd/etcd-raft.yaml >/dev/null <<'EOF'
deployd:
  id: 1
  cluster:
    - http://host1:21521
    - http://host2:21521
    - http://host3:21521
  bind_address: 100.0.0.1:21521
  join: false
job:
  id: 1
  cluster:
    - http://host1:21531
    - http://host2:21531
    - http://host3:21531
  bind_address: 100.0.0.1:21531
  join: false

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
EnvironmentFile=-/etc/deployd/overwrite.env

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

Since it is expected to be run on 3 host, it is OK to have error now.
We can proceed with the next hosts.
After all 3 has been configured, there should be no more error log.

We then can try the endpoint to validate.

```bash
curl -H "X-Namespace: *" "http://host1:9401/deployd/service"
```
