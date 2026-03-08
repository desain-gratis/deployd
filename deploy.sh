# make build 


scp deployd root@mb1:/opt/deployd/releases/0/deployd.tmp &
scp deployd root@mb2:/opt/deployd/releases/0/deployd.tmp &
scp deployd root@mb3:/opt/deployd/releases/0/deployd.tmp

wait

ssh root@mb1 sudo systemctl stop deployd && ssh root@mb1 mv /opt/deployd/releases/0/deployd.tmp /opt/deployd/releases/0/deployd && ssh root@mb1 sudo systemctl start deployd
ssh root@mb2 sudo systemctl stop deployd && ssh root@mb2 mv /opt/deployd/releases/0/deployd.tmp /opt/deployd/releases/0/deployd && ssh root@mb2 sudo systemctl start deployd
ssh root@mb3 sudo systemctl stop deployd && ssh root@mb3 mv /opt/deployd/releases/0/deployd.tmp /opt/deployd/releases/0/deployd && ssh root@mb3 sudo systemctl start deployd
