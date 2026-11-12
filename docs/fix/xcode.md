---
description: "Free space taken by Xcode: delete old DerivedData and iOS DeviceSupport safely with diskdiet, never touching Archives."
---

# Xcode DerivedData and simulators

Xcode's build products (**DerivedData**) and the debug symbols it copies from every
iPhone, Watch or Apple TV you connect (**DeviceSupport**) live in
`~/Library/Developer/Xcode` and can take 20–60 GB.

```sh
npx diskdiet scan --only xcode
npx diskdiet clean --only xcode
```

diskdiet removes DerivedData folders and DeviceSupport versions not changed for 30 days.
Xcode rebuilds DerivedData on the next build and copies DeviceSupport again when you
connect a device with that OS version. **Archives are never touched**: they're your
builds for the App Store.

## Simulators

Old simulator runtimes aren't removed by diskdiet yet. Xcode can do it:

```sh
xcrun simctl delete unavailable     # simulators for runtimes you no longer have
```

or **Xcode → Settings → Platforms** to remove whole runtimes.
