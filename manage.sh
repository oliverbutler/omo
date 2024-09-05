#!/bin/bash

case "$1" in
  local)
    docker-compose -f docker-compose.yml up --build
    ;;
  down)
    docker-compose down
    ;;
  logs)
    docker-compose logs -f
    ;;
  clean)
    docker-compose down -v --remove-orphans
    ;;
  migration)
    docker run --rm -it --network=host -v "$(pwd)/db:/db" ghcr.io/amacneil/dbmate new "$2"
    ;;
  migrate)
    docker run --rm -it --network=host  -v "$(pwd)/db:/db" ghcr.io/amacneil/dbmate --url=postgresql://postgres:password@127.0.0.1/oliverbutler?sslmode=disable up
    ;;
  dbmate)
    docker run --rm -it --network=host -v "$(pwd)/db:/db" ghcr.io/amacneil/dbmate "$2"
    ;;
  *)
    echo "Usage: $0 {local|down|logs|clean|migration|migrate|dbmate}"
    exit 1
esac
