make build 

ssh root@mb2 "sudo systemctl stop deployd"
scp deployd root@mb2:/opt/deployd/releases/0
ssh root@mb2 "sudo systemctl restart deployd"

ssh root@mb3 "sudo systemctl stop deployd"
scp deployd root@mb3:/opt/deployd/releases/0
ssh root@mb3 "sudo systemctl restart deployd"

ssh root@mb1 "sudo systemctl stop deployd"
scp deployd root@mb1:/opt/deployd/releases/0
ssh root@mb1 "sudo systemctl restart deployd"
