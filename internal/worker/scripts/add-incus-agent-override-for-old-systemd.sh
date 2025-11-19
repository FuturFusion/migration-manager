#!/bin/sh

set -e

# Older versions of systemd, such as seen in CentOS7, have a problem with the "-" in the
# incus-agent service defintion's WorkingDirectory. Place an override to work around that.

mkdir -p /etc/systemd/system/incus-agent.service.d/
cat <<EOF > /etc/systemd/system/incus-agent.service.d/override.conf
[Service]
WorkingDirectory=
ExecStart=
ExecStart=/bin/sh -c "cd /run/incus_agent/; exec ./incus-agent"
EOF
