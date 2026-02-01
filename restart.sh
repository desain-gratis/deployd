ssh root@mb1 "sudo systemctl stop deployd & rm -rf /opt/deployd/data/*"
ssh root@mb3 "sudo systemctl stop deployd & rm -rf /opt/deployd/data/*"
ssh root@mb3 "sudo systemctl stop deployd & rm -rf /opt/deployd/data/*"

clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_1_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_1_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_1_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_2_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_2_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_2_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_3_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_3_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `deployd-v1_3_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_1_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_1_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_1_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_2_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_2_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_2_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_3_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_3_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `secretd-v1_3_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_1_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_1_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_1_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_2_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_2_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_2_3`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_3_1`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_3_2`'
clickhouse-client -h clickhouse-darurat --password default -q 'drop database `artifactd-v1_3_3`'

ssh root@mb1 "sudo systemctl restart deployd"
ssh root@mb3 "sudo systemctl restart deployd"
ssh root@mb3 "sudo systemctl restart deployd"