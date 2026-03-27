#!/bin/sh

set -ex

if ! which systemctl > /dev/null 2>&1 ; then
  echo "Skipping agent setup due to missing systemctl"
  exit 0
fi

# Older versions of systemd, such as seen in CentOS7, have a problem with the "-" in the
# incus-agent service defintion's WorkingDirectory. Place an override to work around that.

mkdir -p /etc/systemd/system/incus-agent.service.d/
cat <<EOF > /etc/systemd/system/incus-agent.service.d/override.conf
[Service]
WorkingDirectory=
ExecStart=
ExecStart=/bin/sh -c "cd /run/incus_agent/; exec ./incus-agent"
Type=
Type=simple
EOF

# The abstract unix socket is also unlabeled, so we need to allow initrc communication explicitly.
if getenforce >/dev/null 2>&1 ; then
  cat > /tmp/incus_agent.cil << EOF
(allow initrc_t unlabeled_t (socket (getopt read write ioctl)))
EOF

  semodule -i /tmp/incus_agent.cil
fi
