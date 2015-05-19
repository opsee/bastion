#!/usr/bin/env bash

BARTNET_CONTAINER_NAME=""

while getopts l:h opt; do
  case $opt in
    l)
      BARTNET_CONTAINER_NAME=$OPTARG
      ;;
    h)
      usage
      exit 1
      ;;
  esac
done

usage() {
  cat <<EOF
usage: $0 [-p]

  -l BARTNET_CONTAINER_NAME (postgres)
  -h print this message                       
EOF
}

DOCKER_OPTS="--rm --name bastion "

if [ -n "$BARTNET_CONTAINER_NAME" ]; then
  DOCKER_OPTS="--link $BARTNET_CONTAINER_NAME:bartnet $DOCKER_OPTS" 
fi 

if [ -n "$AWS_ACCESS_KEY_ID" ]; then
  DOCKER_OPTS="$DOCKER_OPTS -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -e AWS_REGION=us-west-1"
fi

set -x
docker run $DOCKER_OPTS opsee/bastion
