export BASTION_VERSION=$(git rev-parse HEAD)
export BASTION_ID=61f25e94-4f6e-11e5-a99f-4771161a3517
docker run --link bastion_slate_1:slate -e "SLATE_HOST=slate:7000" -e "TARGETS=linux/amd64" --link bastion_nsqd_1:nsqd -e "NSQD_HOST=nsqd:4150" -e CUSTOMER_ID -e CUSTOMER_EMAIL -e BARTNET_HOST -e BASTION_AUTH_TYPE -e CUSTOMER_PASSWORD -e BASTION_AUTH_ENDPOINT -v `pwd`:/build quay.io/opsee/build-go:go15
docker build -t quay.io/opsee/bastion:$BASTION_VERSION .
