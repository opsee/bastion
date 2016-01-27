FROM gliderlabs/alpine:3.2

ENV BARTNET_HOST="https://bartnet.in.opsee.com"
ENV NSQD_HOST="nsqd:4150"
ENV SLATE_HOST=""
ENV CA_PATH="ca.pem"
ENV CERT_PATH="cert.pem"
ENV KEY_PATH="key.pem"
ENV CUSTOMER_ID="unknown-customer"
ENV CUSTOMER_EMAIL="unknown-customer-email"
ENV BASTION_AUTH_TYPE="unknown-bastion-auth-type"
ENV BASTION_AUTH_ENDPOINT="https://auth.opsee.com/authenticate/password"
ENV HOSTNAME=""
ENV AWS_ACCESS_KEY_ID=""
ENV AWS_SECRET_ACCESS_KEY=""

RUN apk add --update bash ca-certificates

COPY target/linux/amd64/bin/* /
