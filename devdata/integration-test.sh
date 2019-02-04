#!/bin/bash

docker run \
	--detach \
	--env B2_ACCT_ID="${B2_ACCT_ID}" --env B2_APP_KEY="${B2_APP_KEY}" --env B2_BUCKET="${B2_BUCKET}" \
	--name server \
	--publish 4001:4001 \
	--workdir / \
	server \
	/go/bin/server -alsologtostderr -v=0 -cfg=/devdata/config.dev.yaml

echo -n 'Waiting for server to come up.'
wait=true
while $wait; do
	test=$(docker logs server 2>&1 | fgrep "CT HTTP Server Starting")
	if [ "$test" != "" ]; then
		echo " Success!"
		wait=false
	else
		echo -n "."
		sleep 1
	fi
done
sleep 1

ct-log-tester --uri http://localhost:4001 || docker logs server
