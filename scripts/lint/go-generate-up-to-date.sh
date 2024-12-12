#!/bin/sh -eu

go generate ./...

if [ -n "$(git status --porcelain -- '**/*_gen.go')" ]; then
  git status -- '**/*_gen.go'
  exit 1
fi

exit 0

