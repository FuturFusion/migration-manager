#!/bin/sh

# This is a very naive approach at creating network interface aliases for a newly migrated VM.
# It assumes the order of NICs in the source VM will match the order of the migrated VM.
# The script will look at possible configurations in the following order:
#  1. netplan (Ubuntu, possibly some Debian)
#  2. /etc/NetworkManager/system-connections/*.nmconnection (newer NetworkManager RHEL config)
#  3. /etc/network/interfaces (classic Debian network config)
#  4. /etc/sysconfig/network{,-scripts}/ifcfg-* (older RHEL/SUSE network config)

if [ $# -ne 1 ]; then
  exit 0
fi

hwaddrs="${1}"

process_devs () {
    _device_num=1
    for mac in ${hwaddrs} ; do
        name="$(echo "${1}" | sed -n "${_device_num}p")"
        if [ -z "${name}" ]; then
          break
        fi

        line="SUBSYSTEM==\"net\", ACTION==\"add\", ATTR{address}==\"${mac}\", NAME=\"${name}\""
        conf_file="/etc/udev/rules.d/00-net-symlink.rules"

        if test -e "${conf_file}" && grep -q "${line}" "${conf_file}" ; then
          continue
        fi

        echo "${line}" >> "${conf_file}"
        _device_num=$((_device_num + 1))
    done
}

NETPLAN_DEVS=$(netplan get ethernets | grep -v "^  " | sed -e "s/://")
NETWORKMANAGER_DEVS=$(grep -P -h -o "(?<=interface-name\=).*" /etc/NetworkManager/system-connections/*.nmconnection | sort | uniq | grep -v "^lo$")
NET_INTERFACES_DEVS=$(grep -P -o "(?<=iface ).*(?= inet)" /etc/network/interfaces | sort | uniq | grep -v "^lo$")
# shellcheck disable=SC2046,SC3009
NET_SCRIPTS_DEVS=$(basename -a $(ls /etc/sysconfig/network{,-scripts}/ifcfg-*) | grep -P -o "(?<=ifcfg-).*" | sort | uniq | grep -v "lo$" | grep -v "\.bak$")

if [ ${#NETPLAN_DEVS} -gt 0 ]; then
    process_devs "${NETPLAN_DEVS}"
elif [ ${#NETWORKMANAGER_DEVS} -gt 0 ]; then
    process_devs "${NETWORKMANAGER_DEVS}"
elif [ ${#NET_INTERFACES_DEVS} -gt 0 ]; then
    process_devs "${NET_INTERFACES_DEVS}"
elif [ ${#NET_SCRIPTS_DEVS} -gt 0 ]; then
    process_devs "${NET_SCRIPTS_DEVS}"
fi
