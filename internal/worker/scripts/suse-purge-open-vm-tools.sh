#!/bin/sh

set -e

# Purge VMware tools from the target system.
zypper --no-refresh --ignore-unknown remove -y open-vm-tools open-vm-tools-desktop
