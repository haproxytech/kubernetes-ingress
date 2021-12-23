#! /bin/sh

set -e
if [ -z $SUBJECT ];
then
	SUBJECT="/C=FR/L=PARIS/O=Echo HTTP/CN=$(hostname)"
fi
openssl req -x509 -nodes -days 365 \
	-newkey rsa:2048 \
	-keyout server.key \
	-out server.crt \
	-subj "$SUBJECT"
