#!/bin/sh
set -x
export
cat /etc/hosts
if [ -f /etc/opsee/bastion-env.sh ]; then
	source /etc/opsee/bastion-env.sh
fi

exec /gopath/bin/bastion \
	-opsee=$BARTNET_HOST:$BARTNET_PORT \
	-access_key_id="$AWS_ACCESS_KEY_ID" \
	-secret_key="$AWS_SECRET_ACCESS_KEY" \
	-region=$AWS_REGION \
	-ca=$CA_PATH \
	-cert=$CERT_PATH \
	-key=$KEY_PATH \
	-hostname=$HOSTNAME \
	-customer_id=$CUSTOMER_ID