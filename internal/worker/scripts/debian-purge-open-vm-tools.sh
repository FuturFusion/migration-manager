#!/bin/sh

set -e

# Purge VMware tools from the target system.
apt-get purge -y open-vm-tools open-vm-tools-desktop
apt-get autopurge -y
