HOST=$(ssh localhost -p2200 ls -n | cut -d ',' -f 1)
ssh -J localhost:2200 kill $HOST
GOOS=windows GOARCH=amd64 RSSH_HOMESERVER=192.168.122.1:2200 make  && scp bin/client.exe nhas@192.168.122.78:client.exe
