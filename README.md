# bastion
the real bastion software

## Building

Bastion uses [gb](https://getgb.io) and [go-build](https://github.com/opsee/go-build).

### Compiling locally

`CGO_ENABLED=0 gb build`

Binaries will be under bin/

The bastion, of course, requires you to compile the bastion protobuf spec.
It's recommended to use the build container for that, as you will not have
to deal with dependencies locally.

### Building the container

```
docker pull quay.io/opsee/build-go
docker run -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:latest .
```

### Integration Testing

We do all of our integration testing by building a bastion instance and testing
against it. This is done with [mami](https://github.com/opsee/mami). To
configure mami and run tests against the bastion, do the following.

Set the following environment variables:
* CUSTOMER_ID
* BASTION_ID
* VPN_PASSWORD

These can be obtained from someone or you can setup your own customer/bastion
to use for your local testing. The Vape service handles Bastion authentication,
so you would get all of this from it.

Then, build the bastion container, push your version, and build a bastion
instance.

```bash
# You can version your bastion however you want, for CI builds we do rev hashes.
export BASTION_VERSION=$(git rev-parse HEAD)
docker run -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:$BASTION_VERSION .
aws s3 cp s3://opsee-releases/clj/mami/master/mami-0.1.0-SNAPSHOT-standalone.jar mami/mami.jar
java -jar mami/mami.jar build --bastion-version $BASTION_VERSION build/mami.json

# If you wish to keep your bastion alive after mami is finished, pass the
# --no-cleanup flag to the build command.
# java -jar mami/mami.jar build --no-cleanup --bastion-version $BASTION_VERSION build/mami.json
```
