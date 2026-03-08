ssh root@mb1 "sudo systemctl stop deployd"
ssh root@mb2 "sudo systemctl stop deployd"
ssh root@mb3 "sudo systemctl stop deployd"

ssh root@mb1  "rm -rf /opt/deployd/data/* && rm -rf /data/raft/*"
ssh root@mb2  "rm -rf /opt/deployd/data/* && rm -rf /data/raft/*"
ssh root@mb3  "rm -rf /opt/deployd/data/* && rm -rf /data/raft/*"

clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-3"'

ssh root@mb1 "sudo systemctl restart deployd"
ssh root@mb2 "sudo systemctl restart deployd"
ssh root@mb3 "sudo systemctl restart deployd"
