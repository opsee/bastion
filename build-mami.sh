export BASTION_VERSION=$(git rev-parse HEAD)
docker run -v `pwd`:/build quay.io/opsee/build-go
docker build -t quay.io/opsee/bastion:$BASTION_VERSION .
docker push quay.io/opsee/bastion:$BASTION_VERSION
aws s3 cp s3://opsee-releases/clj/mami/master/mami-0.1.0-SNAPSHOT-standalone.jar mami/mami.jar
java -jar mami/mami.jar build --no-cleanup --bastion-version $BASTION_VERSION build/mami.json 
