#!/bin/sh

set -e

# Purge VMware tools from the target system.
yum erase -y open-vm-tools open-vm-tools-desktop
