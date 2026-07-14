#!/bin/sh
set -e

# Ensure the persisted data subdirectories exist. The app also creates these on
# demand, but pre-creating them keeps a freshly mounted empty volume tidy and
# avoids first-run races. Paths mirror the *_PATH env vars set in the image.
mkdir -p \
    "${STEAMCMD_PATH:-/data/steamcmd}" \
    "${PALWORLD_BASE_PATH:-/data/palworld}" \
    "${LOG_DIR:-/data/logs}"

# The Steam client library symlinks Palworld needs (~/.steam/sdk64/...) are set
# up by the manager after SteamCMD is installed (steamcmd.EnsureSteamClientLinks),
# so nothing to do here. Hand off to the manager binary.
exec "$@"
