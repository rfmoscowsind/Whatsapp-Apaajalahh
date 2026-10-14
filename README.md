# Apaajalahh — WA Native Lite

**Native Windows messaging client experiment: no Chromium, no WebView, no Electron.**
Designed for text chat, WhatsApp groups, and documents on Windows 10/11 x64.

> IMPORTANT: This is an **unofficial WhatsApp Multi-Device client** based on Whatsmeow. Its use may violate WhatsApp's terms, break after protocol changes, or lead to account restrictions. **Use a disposable/test account** before considering any other use. It has not been verified with a real phone.

## Download / start (no Visual Studio required)
1. Open [GitHub Actions](https://github.com/rfmoscowsind/Whatsapp-Apaajalahh/actions) and select the newest successful **Build WA Native Lite (Windows)** run.
2. Download artifact **WA-Native-Lite-win-x64**.
3. Unzip the artifact and then unzip **WA-Native-Lite-win-x64.zip** inside it.
4. Keep \`WANativeLite.exe\` and \`engine/wa-engine.exe\` in their original directory structure.
5. Open \`WANativeLite.exe\`.
6. Click **Link phone (QR)**. On WhatsApp on a **test phone**, open **Linked devices > Link a device** and scan the displayed code.
7. Wait for history sync, select a chat, and send text/attachments (64 MiB per file in this trial). Use **+ Chat** for a phone number including country code, such as \`628xxxxxxxxx\`.

Do not download the Windows executable from untrusted third-party sites. Builds are unsigned, and Windows SmartScreen may warn.

## Current implementation
- WPF/C# native Windows UI and separate Go/Whatsmeow engine
- Session re-use through Whatsmeow SQLite store, QR pairing with expiring QR rotation
- Cache-first conversation and message history (up to 300 messages per opened chat)
- Direct chats, group history/name sync, send and receive text
- Upload documents, retrieve supported media with Save As
- New chat by phone number, search cached chats, logout and cache clearing
- Engine-to-UI JSON-line IPC using inherited standard input/output (no HTTP server/listening port)
- Native packaged portable build with GitHub Actions, Windows runner and engine unit tests

## Current gaps / cautions
- **No calls, video calls, or screen sharing (by design).**
- No full support for stickers, mentions, read receipts, typing UI, reactions, edits, disappearing message deletion, status and channels. Unsupported types show placeholders.
- No automatic image previews or auto-download; media downloaded only at user request.
- Sent files use WhatsApp **Document** payload, not gallery/photo mode.
- WhatsApp may reject or alter behavior of unofficial companion clients at any time. **Server compatibility and QR authentication have not been end-to-end tested yet.**
- **Local SQLite cache and session are NOT encrypted at rest**. Windows user-profile isolation is not a replacement for full-disk encryption. Prefer BitLocker/device encryption, do not share your Windows account, and do not use sensitive/main-account content for this experiment. Session keys, cached chats and message metadata can be found in \`%APPDATA%\WA-Native-Lite\` (Go's user config directory).
- Logout clears local cached conversations. Existing files exported outside the app are NOT deleted automatically.
- The UI and engine are separate processes. If either executable is missing, the application cannot connect.
- This release has not been code-signed or independently security-audited.
- Max upload is 64 MiB to cap in-memory pressure; downloads currently load decrypted media into engine memory before saving.

## Developer build
- Windows 10/11, .NET 10 SDK, Go 1.26.5.
- Run \`cd engine && go mod tidy && go test ./... && go build -o ../out/engine/wa-engine.exe .\`
- Run \`dotnet publish desktop/WANativeLite.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -o out\`
- Launch \`out/WANativeLite.exe\`.

## Layout
- \`desktop/\` — WPF UI (C#, XAML)
- \`engine/\` — local Whatsmeow engine (Go)
- \`.github/workflows/build-windows.yml\` — test, compile, verify, zip and upload Windows artifact

## Trial checklist
- [ ] Open cold-start and measure RAM (Task Manager for both processes)
- [ ] QR pairing succeeds with test phone
- [ ] Restart retains pairing
- [ ] History and group titles sync after linking
- [ ] Receive and send direct and group text
- [ ] Send PDF/document to test chat
- [ ] Download and open received attachment
- [ ] Disconnect network, restore connectivity, and check reconnect
- [ ] Logout, restart, pair again
- [ ] Validate privacy of session DB and exported files

Do not treat passing CI compilation as a guarantee the live WhatsApp protocol works.
