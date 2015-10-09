export BASTION_VERSION=$(git rev-parse HEAD)
export BASTION_ID=61f25e94-4f6e-11e5-a99f-4771161a3517
docker run -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:$BASTION_VERSION .
docker push quay.io/opsee/bastion:$BASTION_VERSION
aws s3 cp s3://opsee-releases/clj/mami/master/mami-0.1.0-SNAPSHOT-standalone.jar mami/mami.jar
java -jar mami/mami.jar build --no-cleanup --bastion-version $BASTION_VERSION build/mami.json

