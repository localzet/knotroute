# Android client

KnotRoute's Android client embeds the same Go networking core used by desktop/server deployments. It does not depend on the standalone desktop client and does not create an Android VPN/TUN interface.

## Components

```text
android/app                  Kotlin application / WebView UI
mobile/knotmobile            gomobile-facing Go API
pkg/knotclient               embeddable Go client core
```

`gomobile bind` turns `mobile/knotmobile` into `knotroute-client.aar`.

The app starts the core from a foreground service and configures AndroidX WebKit `ProxyController` so only that application process uses the loopback KnotRoute HTTP proxy.

## Requirements

The repository targets:

- JDK 17;
- Android Gradle Plugin 9.3.0;
- Gradle 9.5.0;
- compile SDK 37;
- target SDK 36;
- min SDK 26;
- AndroidX WebKit 1.16.0;
- current `gomobile` / `gobind` compatible with the configured Go toolchain.

GitHub Actions installs these prerequisites automatically.

## Build on Linux/macOS

Set `ANDROID_HOME` and make sure `sdkmanager` has the platform/build-tools declared in `.github/workflows/android.yml`.

Install gomobile:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
```

Provide Gradle 9.5+ on `PATH`, then:

```bash
./scripts/build-android.sh
```

Outputs:

```text
dist/knotroute_3.0.0_android_debug.apk
dist/knotroute_3.0.0_android_release-unsigned.apk
dist/knotroute-client_3.0.0_android.aar
```

The debug APK is automatically debug-signed by the Android toolchain. The release APK is intentionally unsigned; sign it with your own release keystore before distribution.

## Build on Windows

```powershell
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
$env:ANDROID_HOME = "C:\Users\you\AppData\Local\Android\Sdk"
.\scripts\build-android.ps1
```

Android Studio can also open the `android/` directory after `android/app/libs/knotroute-client.aar` has been generated.

## CA installation

The embedded core generates a device-local Root CA in the application's private files directory.

The **CA** action invokes Android's normal `KeyChain.createInstallIntent()` flow. Android requires user interaction; KnotRoute does not and cannot silently add a root certificate.

The app's `network_security_config.xml` explicitly allows user-installed roots so its own WebView can trust the installed KnotRoute CA on Android versions where user roots are not trusted by applications by default.

TLS errors in WebView are cancelled; the app never calls `SslErrorHandler.proceed()`.

## Embedding the AAR in another Android application

The AAR exposes a gomobile-compatible `Client` object through the generated `knotmobile` package.

Conceptually:

```text
CreateClient(optionsJson)
Start()
NodeAddress()
HTTPProxyURL()
RootCAPEM()
OpenForward(serviceAddress) -> local TCP port
Stop()
```

This allows an application to ship KnotRoute connectivity itself. A messaging client, for example, can call `OpenForward()` for a remote `.knot` endpoint and point its existing TCP protocol implementation at `127.0.0.1:<returned-port>`.

The standalone KnotRoute Android app is therefore optional when an application embeds the AAR.

## No TUN by default

Because the Android implementation uses an in-process/loopback proxy rather than `VpnService`, it can coexist with another VPN/TUN owner. Applications that embed the AAR can use direct `OpenForward` streams and avoid proxy configuration entirely.
