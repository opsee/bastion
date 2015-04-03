#!/bin/bash

dir="$1/creds"
serial=`cat ${1}/ca.db.serial`
password="password"

echo $serial

openssl genrsa -des3 -passout "pass:${password}" -out "$dir/${serial}-key.pem" 1024
openssl rsa -in "$dir/${serial}-key.pem" -out "$dir/${serial}-key-un.pem" -passin "pass:$password"
mv "$dir/$serial-key-un.pem" "$dir/$serial-key.pem"
openssl req -new -days 1095 -key "$dir/$serial-key.pem" -out "$dir/$serial-csr.pem" -config openssl.cnf
openssl ca -config openssl.cnf -name "CA_$1" -out "$dir/$serial-cert.pem" -infiles "$dir/$serial-csr.pem"
