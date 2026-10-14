# Whatsapp Apaajalahh (WA Native Lite)

Experimental **native Windows** messaging desktop application focused on instant startup, chat, and file attachments.

> **Current status:** Offline UI prototype. This build DOES NOT connect to WhatsApp, authenticate an account, or send actual messages/files.

## Technology
- C# / WPF / .NET 8; no Chromium or WebView.
- Windows x64 self-contained publish on GitHub Actions.
- In-memory demo conversations, sending demo text, and selecting attachments in the UI.

## Build
1. Visit **Actions** > **Build Windows**.
2. Run workflow (or push to `main`).
3. Download the `WA-Native-Lite-win-x64` artifact.
4. Extract the zip and run `WANativeLite.exe` on Windows 10/11 x64.

Locally: `dotnet publish desktop/WANativeLite.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true`

## Roadmap
1. UI and startup/RAM benchmarks.
2. Encrypted local cache and secure session storage.
3. A **separately reviewed**, opt-in unofficial WhatsApp Multi-Device engine, tested only with a disposable test account.
4. Actual message and media transfer, sync recovery, diagnostics.

**Caution:** Unofficial WhatsApp clients may violate WhatsApp terms, stop functioning after protocol changes, or cause account restrictions. Do not use a primary account for experiments.

The WPF approach is deliberate for a minimal reproducible Windows build. A WinUI 3 UI can be evaluated later against benchmarks.
