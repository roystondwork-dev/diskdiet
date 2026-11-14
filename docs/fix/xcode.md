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

```sh
npx diskdiet clean --only simulators
```

removes simulators for iOS, watchOS and tvOS versions that are no longer installed
(diskdiet runs Xcode's own `xcrun simctl delete unavailable`) and simulator caches not
used for 30 days. Simulators you can still run are never touched. To remove whole
runtimes you no longer need, use **Xcode → Settings → Platforms**.

## CocoaPods and Swift packages

```sh
npx diskdiet clean --only cocoapods,swiftpm
```

removes old pod versions from CocoaPods' download cache and old Swift Package Manager
repositories and artifacts. Your projects' `Pods` folders and `Package.resolved` are never
touched.
