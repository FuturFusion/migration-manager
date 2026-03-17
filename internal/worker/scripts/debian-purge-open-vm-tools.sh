#!/bin/sh

set -e

# Purge VMware tools from the target system.
if dpkg -l | grep -q "open-vm-tools-desktop" ; then
  apt-get purge -y open-vm-tools-desktop
fi

if dpkg -l | grep -q "open-vm-tools" ; then
  apt-get purge -y open-vm-tools
fi

apt-get autopurge -y
