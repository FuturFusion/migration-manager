#!/bin/sh

# Mask the lxd-agent service.
if systemctl list-unit-files --type=service | grep -q "lxd-agent.service" ; then
  systemctl mask lxd-agent.service
fi

# If the incus agent already exists then there's nothing we need to do.
if systemctl list-unit-files --type=service | grep -q "incus-agent.service" ; then
  exit 0
fi

# Install incus-agent into the target system.
mkdir -p /mnt/config/
mount -t 9p config /mnt/config/
cd /mnt/config/ || exit 1
./install.sh
cd /root || exit 1
umount /mnt/config/

# SELinux handling.
if getenforce >/dev/null 2>&1 ; then
  if ! type semanage >/dev/null 2>&1  ; then
    # If semanage is not available, manually write the labels.
    echo "/var/run/incus_agent/incus-agent    system_u:object_r:bin_t:s0" >> /etc/selinux/targeted/contexts/files/file_contexts.local
    echo "/usr/lib/systemd/incus-agent-setup    system_u:object_r:init_exec_t:s0" >> /etc/selinux/targeted/contexts/files/file_contexts.local
    sefcontext_compile -o /etc/selinux/targeted/contexts/files/file_contexts.local.bin /etc/selinux/targeted/contexts/files/file_contexts.local

    chcon system_u:object_r:file_context_t:s0 /etc/selinux/targeted/contexts/files/file_contexts.local
    chcon system_u:object_r:file_context_t:s0 /etc/selinux/targeted/contexts/files/file_contexts.local.bin
  else
    # Add labels for the binaries and scripts executed by the incus-agent service.
    semanage fcontext -N -a -t bin_t /var/run/incus_agent/incus-agent
    semanage fcontext -N -a -t init_exec_t /usr/lib/systemd/incus-agent-setup
  fi

  # Manually set the label for the file we already created because restorecon doesn't work in chroot.
  chcon system_u:object_r:init_exec_t:s0 /usr/lib/systemd/incus-agent-setup
fi
