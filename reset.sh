ssh root@mb1 "sudo systemctl stop deployd"
ssh root@mb3 "sudo systemctl stop deployd"
ssh root@mb3 "sudo systemctl stop deployd"

ssh root@mb1  "rm -rf /opt/deployd/data/*"
ssh root@mb2  "rm -rf /opt/deployd/data/*"
ssh root@mb3  "rm -rf /opt/deployd/data/*"

clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_1_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_1_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_1_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_2_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_2_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_2_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_3_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_3_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deployd-v1_3_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_1_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_1_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_1_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_2_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_2_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_2_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_3_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_3_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "secretd-v1_3_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_1_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_1_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_1_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_2_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_2_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_2_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_3_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_3_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "artifactd-v1_3_3"'

clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_3"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_1"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_2"'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database IF EXISTS "deploy-job-v1_4_3"'

ssh root@mb1 "sudo systemctl restart deployd"
ssh root@mb3 "sudo systemctl restart deployd"
ssh root@mb3 "sudo systemctl restart deployd"