#!/bin/bash
openssl ecparam -genkey -name secp256r1 -noout | openssl pkey > valid_private_key.pem
openssl ecparam -genkey -name secp521r1 -noout | openssl pkey > invalid_curve.pem
openssl ecparam -genkey -name secp256r1 -noout > not_pkcs8.pem
openssl genrsa | openssl pkey > invalid_rsa.pem
openssl ec -in valid_private_key.pem -pubout > public_key.pem
touch empty.pem
