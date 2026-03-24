#!/bin/sh

set -ex

# Purge VMware tools from the target system.
yum erase -y open-vm-tools open-vm-tools-desktop || yum remove -y open-vm-tools open-vm-tools-desktop
