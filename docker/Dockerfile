FROM kiasaki/alpine-golang

ENV BARTNET_HOST="bartnet"
ENV BARTNET_PORT="4080"
ENV AWS_ACCESS_KEY_ID=""
ENV AWS_SECRET_ACCESS_KEY=""
ENV AWS_REGION=""

RUN apk --update add build-base

RUN mkdir -p /gopath/src/github.com/opsee/bastion

COPY . /gopath/src/github.com/opsee/bastion
RUN cd /gopath/src/github.com/opsee/bastion && \
    make

ADD docker/bin/start.sh /
RUN apk del build-base

ENTRYPOINT /start.sh
