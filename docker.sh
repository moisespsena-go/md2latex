#!/usr/bin/env bash
name="moisespsena/md2latex"

case "$1" in
build)
  cat Dockerfile.template | perl -pe 's/__HASH__/'`git log -1 --pretty=format:'%H'`'/g' > Dockerfile || exit $?
  docker build -t $name .
  ;;

run)
  docker run $name
  ;;

push)
  docker login && \
  docker push $name
  ;;
esac