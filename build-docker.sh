export BASTION_VERSION=$1
export BASTION_ID=61f25e94-4f6e-11e5-a99f-4771161a3517
make clean
docker run --link nsqd:nsqd -e CUSTOMER_EMAIL -e BASTION_AUTH_TYPE -e CUSTOMER_ID -e CUSTOMER_PASSWORD -e BARTNET_HOST -e BASTION_AUTH_ENDPOINT -e "NSQD_HOST=nsqd:4150" -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:$1 .
docker push quay.io/opsee/bastion:$1
