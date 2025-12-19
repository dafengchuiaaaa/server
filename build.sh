#!/bin/bash

# Usage: ./build.sh [prod|test]
# prod: deploy to production (124.221.66.50)
# test: deploy to test (1.94.173.175)

set -e

ENV=${1:-test}
MAIN_GO="./cmd/main.go"

# Redis configurations
TEST_REDIS_ADDR="192.168.0.147:6379"
TEST_REDIS_PASS="W3gS3nslOOrRqRa6"
PROD_REDIS_ADDR="10.0.4.12:6379"
PROD_REDIS_PASS="G20pRGLObzkXKkzL"

# Deploy targets
TEST_HOST="1.94.173.175"
PROD_HOST="124.221.66.50"

switch_to_test() {
    # Use comment to locate the line
    sed -i '' "s|Addr:.*// Redis服务端地址|Addr:     \"${TEST_REDIS_ADDR}\", // Redis服务端地址|" "$MAIN_GO"
    sed -i '' "s|Password:.*// Redis服务端的密码|Password: \"${TEST_REDIS_PASS}\",   // Redis服务端的密码|" "$MAIN_GO"
}

switch_to_prod() {
    # Use comment to locate the line
    sed -i '' "s|Addr:.*// Redis服务端地址|Addr:     \"${PROD_REDIS_ADDR}\",   // Redis服务端地址|" "$MAIN_GO"
    sed -i '' "s|Password:.*// Redis服务端的密码|Password: \"${PROD_REDIS_PASS}\", // Redis服务端的密码|" "$MAIN_GO"
}

if [ "$ENV" = "prod" ]; then
    echo "==> Switching to PRODUCTION Redis config"
    switch_to_prod
    DEPLOY_HOST=$PROD_HOST
elif [ "$ENV" = "test" ]; then
    echo "==> Switching to TEST Redis config"
    switch_to_test
    DEPLOY_HOST=$TEST_HOST
else
    echo "Unknown environment: $ENV"
    echo "Usage: ./build.sh [prod|test]"
    exit 1
fi

echo "==> Building project"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o ./cmd/mqtt ./cmd

echo "==> Deploying to $DEPLOY_HOST"
scp -r ~/site-home/github/mqtt-server/cmd/mqtt root@${DEPLOY_HOST}:/home/mqtt

echo "==> Done!"
