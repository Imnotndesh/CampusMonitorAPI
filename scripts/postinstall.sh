#!/bin/bash
set -e

# Create user and group if not exist
if ! getent group campus-monitor >/dev/null; then
    groupadd --system campus-monitor
fi
if ! getent passwd campus-monitor >/dev/null; then
    useradd --system --no-create-home --home-dir /var/lib/campus-monitor --gid campus-monitor --shell /bin/false campus-monitor
fi

# Create log directory (if your app logs to /var/log)
mkdir -p /var/log/campus-monitor-api
chown campus-monitor:campus-monitor /var/log/campus-monitor-api
chmod 750 /var/log/campus-monitor-api

# Create config directory if not exists
mkdir -p /etc/campus-monitor-api
# Copy .env.example to .env if not already present
if [ ! -f /etc/campus-monitor-api/.env ]; then
    cp /usr/share/campus-monitor-api/.env.example /etc/campus-monitor-api/.env
    echo "Created default .env file at /etc/campus-monitor-api/.env – please edit it with your actual values."
fi
# Ensure the service is enabled and started
systemctl daemon-reload
systemctl enable campus-monitor-api.service
systemctl start campus-monitor-api.service || true

echo "Campus Monitor API installed successfully."