#!/bin/sh

set -ex

efi="$(find /boot -iname \*bootx64.efi)"
if [ -z "${efi}" ]; then
  grub-install --removable
fi
