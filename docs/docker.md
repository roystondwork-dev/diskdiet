# Docker and disk space

`diskdiet clean` asks Docker to remove:

- stopped containers created more than `--older-than` ago,
- images no container uses, created more than `--older-than` ago,
- build cache not used for `--older-than`,
- with `--volumes` only: every volume no container uses. Volumes have no age
  filter and can hold databases, so diskdiet never removes them unless you ask.

These are Docker's own `container prune`, `image prune --all`, `builder prune --all`
and `volume prune --all` commands with an `until` filter, run against your current
docker context (Docker Desktop, OrbStack, Colima, Rancher Desktop or Docker Engine).

Note that Docker's age filter for images is the image's **creation** time, which
is when it was built rather than when you pulled it. An image you pulled today can
be older than 30 days, and if no container uses it, it counts as old.

## Docker Desktop's disk file

On macOS and Windows, Docker Desktop keeps everything in a virtual disk file. After
pruning, the space is free inside that disk, but the file may not shrink right away.

**macOS:** Docker Desktop hands freed space back to macOS by itself, which can take
a few minutes. To cap the disk's size, open Docker Desktop → **Settings → Resources
→ Advanced → Virtual disk limit**.

**Windows (WSL 2):** the disk file (for example
`%LOCALAPPDATA%\Docker\wsl\disk\docker_data.vhdx`; older versions use
`...\wsl\data\ext4.vhdx`) does not shrink by itself. To compact it:

1. Quit Docker Desktop, then run `wsl --shutdown`.
2. Compact the file, either:
   - on Windows Pro or Enterprise, in an administrator PowerShell:
     `Optimize-VHD -Path "<path to the .vhdx>" -Mode Full`
   - on any edition, in an administrator prompt:
     ```
     diskpart
     select vdisk file="<path to the .vhdx>"
     attach vdisk readonly
     compact vdisk
     detach vdisk
     exit
     ```
3. Start Docker Desktop again.
