---
description: "Free space taken by the Android SDK: remove emulator system images no AVD uses and older NDK and build-tools versions, safely, with diskdiet."
---

# Android SDK is huge

The Android SDK grows with every emulator image, NDK and build-tools version Android
Studio downloads: 20–50 GB is common, and most of it is versions nothing uses anymore.

```sh
npx diskdiet scan --only android
npx diskdiet clean --only android
```

diskdiet removes, from the SDK (`ANDROID_HOME`, or Android Studio's default location):

- **system images** (`system-images/<api>/<variant>/<abi>`) not changed for 30 days that
  **no emulator uses**: it reads every AVD's `config.ini` and always keeps the images they
  point at;
- **older NDK versions** (`ndk/<version>`): the newest always stays;
- **older build-tools versions**: the newest always stays.

Your emulators (AVDs) themselves are never touched. If a project needs a removed NDK or
build-tools version, Gradle or Android Studio downloads it again; a removed system image
comes back from **Device Manager** or the SDK Manager.

!!! tip
    `npx diskdiet clean --only android --older-than 7d` removes more; `--all` removes every
    unused image and every NDK and build-tools version but the newest.
