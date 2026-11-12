---
description: "Docker using too much disk space? Remove unused images, stopped containers and build cache older than 30 days, safely, with diskdiet."
---

# Docker uses too much disk

Docker keeps every image you pulled or built, every stopped container and the build
cache until you prune them. 20–50 GB is common.

```sh
npx diskdiet scan --only docker
npx diskdiet clean --only docker
```

diskdiet asks Docker to remove, with Docker's own `prune` commands and an age filter:

- stopped containers created more than 30 days ago,
- images no container uses, created more than 30 days ago,
- build cache not used for 30 days.

**Volumes stay** unless you add `--volumes`: they can hold databases.

It works with Docker Desktop, OrbStack, Colima, Rancher Desktop and Docker Engine (it
uses your current `docker context`).

## The space didn't come back?

Docker Desktop keeps everything in a virtual disk file that may not shrink right away,
especially on Windows. See [Docker details](../docker.md#docker-desktops-disk-file) for
how to compact it.

## The equivalent by hand

```sh
docker container prune --filter until=720h
docker image prune --all --filter until=720h
docker builder prune --all --filter until=720h
```

diskdiet runs these for you, shows what can be freed first, and lists Docker next to
everything else filling your disk.
