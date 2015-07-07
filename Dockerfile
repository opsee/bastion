FROM kiasaki/alpine-golang

ENV BARTNET_HOST="api-beta.opsee.co"
ENV BARTNET_PORT="4080"
ENV AWS_ACCESS_KEY_ID=""
ENV AWS_SECRET_ACCESS_KEY=""
ENV AWS_REGION=""
ENV CA_PATH=""
ENV CERT_PATH=""
ENV KEY_PATH=""
ENV HOSTNAME=""
ENV CUSTOMER_ID=""

ADD target/linux/cmd/ /

ADD docker/bin/start.sh /

ENTRYPOINT /start.sh
