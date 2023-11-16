#!/bin/bash

set -e

FILE="$1"
PROJECT="${2:-$FILE}"

docker-compose -f .github/components-scripts/infrastructure/docker-compose-${FILE}.yml -p ${PROJECT} up -d
