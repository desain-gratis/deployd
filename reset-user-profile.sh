


ssh root@mb1 "sudo systemctl stop deployd_user-profile"
ssh root@mb2 "sudo systemctl stop deployd_user-profile"
ssh root@mb3 "sudo systemctl stop deployd_user-profile"

ssh root@mb1 "rm -rf /data/raft/deployd_user-profile/*"
ssh root@mb2 "rm -rf /data/raft/deployd_user-profile/*"
ssh root@mb3 "rm -rf /data/raft/deployd_user-profile/*"

clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "user-profile-1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "user-profile-2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "user-profile-3"'

ssh root@mb1 "sudo systemctl start deployd_user-profile"
ssh root@mb2 "sudo systemctl start deployd_user-profile"
ssh root@mb3 "sudo systemctl start deployd_user-profile"
