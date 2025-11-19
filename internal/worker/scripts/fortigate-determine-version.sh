#!/bin/sh

set -e

conf_gz="/run/mount/target/config/sys_global.conf.gz"
if ! test -L "${conf_gz}" ; then
  exit 1
fi

real_path="$(basename "$(readlink -m "${conf_gz}")")"
if [ -z "${real_path}" ]; then
  exit 1
fi

real_path="/run/mount/target/config/${real_path}"
if ! test -e "${real_path}" ; then
  exit 1
fi

header="$(gzip -dkc "${real_path}" | head -1)"

if [ -z "${header}" ] || ! echo "${header}" | grep -q "^#config-version=" ; then
  exit 1
fi

version="$(echo "${header}" | cut -d'-' -f3)"
if echo "${version}" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ; then
  printf "%s" "${version}"
else
  exit 1
fi
