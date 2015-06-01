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

RUN apk --update add build-base

RUN mkdir -p /gopath/src/github.com/opsee/bastion

COPY . /gopath/src/github.com/opsee/bastion

RUN go get github.com/opsee/bastion
RUN cd /gopath/src/github.com/opsee/bastion && \
    make

ADD docker/bin/start.sh /
RUN apk del build-base

ENTRYPOINT /start.sh
