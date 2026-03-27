#!/bin/sh

set -ex

# Purge VMware tools from the target system.
if zypper --no-refresh --ignore-unknown remove -y open-vm-tools open-vm-tools-desktop 2>&1 | grep -q "Unknown option '--ignore-unknown'" ; then
  if zypper --no-refresh search --installed-only | grep -q "open-vm-tools-desktop" ; then
    zypper --no-refresh remove -y open-vm-tools-desktop
  fi

  if zypper --no-refresh search --installed-only | grep -q "open-vm-tools" ; then
    zypper --no-refresh remove -y open-vm-tools
  fi
fi
