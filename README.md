# Deployd

deploy golang service as ubuntu systemd service

# Installation

This guide describes how to manually install **deployd** as a systemd service.

## Requirements

Tested for Ubuntu.

* Root or sudo access
* 3 hosts/PC in with private static internal IP
* cloudflared
* nginx unit

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

## 1. Download the Release & extract

Download the latest release from GitHub.

```bash
wget https://github.com/desain-gratis/deployd/releases/download/v0.0.16/deployd-linux-amd64.tar.gz -O deployd-linux-amd64.tar.gz && \
sudo mkdir -p /opt/deployd && \
sudo tar -xzf deployd-linux-amd64.tar.gz -C /opt/deployd && \
sudo chmod +x /opt/deployd/deployd

```

The resulting layout should look like:

```text
/opt/deployd/
└── deployd
```

---

## 2. Create the Environment and Secret File Placeholder

```bash
sudo mkdir -p /etc/deployd

sudo tee /etc/deployd/overwrite.env >/dev/null <<'EOF'
CONFIG=/etc/deployd/config.yaml
SECRET=/etc/deployd/secret.yaml
EOF

sudo tee /etc/deployd/secret.yaml >/dev/null <<'EOF'

EOF
```

## 3. Create the config file

Next, create the config. Please adjust it according to your own host information. Explanation below the example.

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
  base_node_host_dir: /data
  base_wal_dir: /data
  etcd_config: /etc/deployd/etcd-raft.yaml

http:
  public:
    address: 100.0.0.1:9401
    fqdn: http://host1.com:9401

ui:
  dir: /var/www

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
    config-data: /data/deployd/config.db
    job-data: /data/deployd/job.db
    job-website-data: /data/deployd/job-website-data.db

EOF

```

_http bind address_ is where the deployd API HTTP interface actually bind to.

http bind address = http://100.0.0.1:9401

_fqdn_
is used to access the HTTP endpoint from external. 
Usually, I configure the load balancer with one single domain that will go to my 3 hosts. Or, you can just put one of the host.

fqdn = http://mycluster.com 

or 

fqdn = http://100.0.0.1:9401

Hosts:
* hostname = host1, hostname_internal_ip = 100.0.0.1, host_id: 1
* hostname = host2, hostname_internal_ip = 100.0.0.2, host_id: 2
* hostname = host3, hostname_internal_ip = 100.0.0.3, host_id: 3

## 4. Create the Etcd Raft config file

Etcd raft is what makes this a distributed application.

```bash
sudo tee /etc/deployd/etcd-raft.yaml >/dev/null <<'EOF'
node_id: 1
base_wal_dir: /wal
base_data_dir: /data
replica:
  deployd:
    cluster:
      - http://host1:21521
      - http://host2:21522
      - http://host3:21523
    bind_address: 100.0.0.1:21521
    join: false
  job:
    cluster:
      - http://host1:21531
      - http://host2:21532
      - http://host3:21533
    bind_address: 100.0.0.1:21531
    join: false
  job-website:
    cluster:
      - http://host1:21541
      - http://host2:21542
      - http://host3:21543
    bind_address: 100.0.0.1:21541
    join: false

EOF
```

## 5. Create the systemd Service

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

Reload systemd daemon, enable to run on server start up, and start deployd

```bash
sudo systemctl daemon-reload && sudo systemctl enable deployd && sudo systemctl start deployd
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
