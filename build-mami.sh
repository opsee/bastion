./build-docker.sh
aws s3 cp s3://opsee-releases/clj/mami/master/mami-0.1.0-SNAPSHOT-standalone.jar mami/mami.jar
java -jar mami/mami.jar build --release testing --bastion-version $BASTION_VERSION --build-ami build/mami.json
