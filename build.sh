
echo "构建项目"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o ./cmd/mqtt ./cmd

# echo "部署项目到新项目"
scp -r ~/site-home/github/mqtt-server/cmd/mqtt root@1.94.173.175:/home/mqtt


