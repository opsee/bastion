#!/bin/sh
set -x
export
cat /etc/hosts
exec /gopath/bin/bastion -opsee=$BARTNET_HOST:$BARTNET_PORT -access_key_id="$AWS_ACCESS_KEY_ID" -secret_key="$AWS_SECRET_ACCESS_KEY" -region=$AWS_REGION
