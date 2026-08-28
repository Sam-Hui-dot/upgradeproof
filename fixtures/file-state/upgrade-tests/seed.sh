#!/bin/sh
set -eu
curl -fsS -X POST --data 'upgradeproof-state' http://127.0.0.1:18081/seed
