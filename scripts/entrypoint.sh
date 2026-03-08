#!/bin/sh

echo "Setting up permissions..."
chown -R hytale:hytale /data

echo "Starting the server binary as hytale user"
exec su-exec hytale /opt/hytale/bin/server