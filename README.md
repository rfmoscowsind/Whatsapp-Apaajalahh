# Apaajalahh — ZapFast track

Native Windows WhatsApp companion client with no browser engine.

**Current main track adopts [ZapFast](https://github.com/crmne/zapfast), a Rust/egui client**, instead of adding more features to the earlier WPF + Go prototype.

## Run on Windows 10/11 x64

1. Open [Build Apaajalahh - ZapFast Windows](https://github.com/rfmoscowsind/Whatsapp-Apaajalahh/actions/workflows/build-zapfast.yml).
2. Pick the newest successful run, download the artifact named **Apaajalahh-ZapFast-win-x64**.
3. Extract the artifact and then the portable ZIP inside it.
4. Run **zapfast.exe**, scan its QR using WhatsApp on your phone → Linked devices → Link a device.
5. For an installer instead, use [ZapFast's official Windows downloads](https://zapfast.rocks/download/#windows).

**This is the unmodified official ZapFast 0.19.0 Windows executable, NOT YET a custom-branded source fork.** Our workflow downloads the official portable release, verifies a pinned SHA-256 digest and its executable/license contents, then publishes the unchanged ZIP. This avoids unnecessary duplicate compilation. The source remains at [crmne/zapfast](https://github.com/crmne/zapfast); any future changes to the UI or code must be made on a source branch/fork and compiled separately.

The GitHub repository's earlier code is preserved in the [legacy-wpf-go-v0.2 branch](https://github.com/rfmoscowsind/Whatsapp-Apaajalahh/tree/legacy-wpf-go-v0.2). The old \`desktop/\` and \`engine/\` folders are retained, but their GitHub Action now runs only when manually dispatched.

**Your existing WPF + Whatsmeow login cannot be imported** into ZapFast. Both apps maintain separate databases and linked-device sessions; scan the QR again. Do not delete old app data without checking it.

## Security & license

ZapFast is unofficial and unaffiliated with WhatsApp / Meta. Unofficial clients may break or cause account restrictions. Test with a disposable number first.

ZapFast © 2026 Carmine Paolino and contributors, licensed [MIT](https://github.com/crmne/zapfast/blob/main/LICENSE). Keep the included LICENSE and THIRD-PARTY-NOTICES with any redistributed copies; do not present the upstream binary as our original work.

## Provenance

- Upstream: **ZapFast v0.19.0**
- Upstream commit: \`6f3766a38328ad081120c91e95ae68cc7e7a3466\`
- Official archive: \`zapfast-v0.19.0-x86_64-pc-windows-msvc.zip\`
- Official archive SHA-256: \`389d1456f258977fdf3c6d93da441d925d6e33ca7a5039a5b0159a0c000bad37\`

Next milestone: optional source fork and custom branding, preserving upstream attribution and update compatibility.
