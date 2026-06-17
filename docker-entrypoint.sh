#!/bin/sh
# Seed the persistent data volume on first run, then start the bot.
#
# Seed files (journals.csv, promotion.yaml, …) are baked into the image at
# /app/seed. The writable data dir (/app/data) is a Docker volume so admin edits
# (menu.yaml, bot.yaml, announcements.yaml) survive redeploys. On each start we
# copy any seed file that is MISSING from the volume — never overwriting an
# admin-edited file.
set -e

mkdir -p /app/data
if [ -d /app/seed ]; then
  for src in /app/seed/* /app/seed/.[!.]*; do
    [ -e "$src" ] || continue
    name=$(basename "$src")
    if [ ! -e "/app/data/$name" ]; then
      cp -r "$src" "/app/data/$name"
    fi
  done
fi

exec /app/manhal "$@"
