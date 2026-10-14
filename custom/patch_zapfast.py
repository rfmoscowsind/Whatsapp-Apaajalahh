#!/usr/bin/env python3
"""Reproducible Apaajalahh changes to pinned ZapFast v0.19.0.

No additional Python packages. Each replacement must match the reviewed source
exactly; if upstream changes, fail the build instead of silently mispatching.
"""
from pathlib import Path
import sys

root = Path(sys.argv[1])
assert (root / "Cargo.lock").is_file(), "Pass the checked-out ZapFast root"

def change(filename: str, old: str, new: str, count: int = 1) -> None:
    path = root / filename
    content = path.read_text(encoding="utf-8")
    actual = content.count(old)
    if actual != count:
        raise SystemExit(f"Refusing patch: {filename} expected {count} matches, found {actual}: {old[:80]!r}")
    path.write_text(content.replace(old, new), encoding="utf-8", newline="\n")
    print(f"Patched {filename} ({count} occurrence(s))")

# Fix upstream #369: Some(0) means unknown read time, not a real read event.
# This must be done at protocol history interpretation, NOT in tick painting.
worker = "src/backend/worker.rs"
change(worker,
       ".any(|receipt| receipt.read_timestamp.is_some())",
       ".any(|receipt| receipt.read_timestamp.is_some_and(|at| at > 0))")
change(worker,
       ".any(|receipt| receipt.receipt_timestamp.is_some())",
       ".any(|receipt| receipt.receipt_timestamp.is_some_and(|at| at > 0))")
change(worker,
       "message.receipts.iter().filter_map(at).min()",
       "message.receipts.iter().filter_map(at).filter(|at| *at > 0).min()")
change(worker,
       "if let Some(at) = at {\n                    rows.push((recipient.clone(), status, at));",
       "if let Some(at) = at.filter(|at| *at > 0) {\n                    rows.push((recipient.clone(), status, at));")

# Undo invalid blue ticks filed by affected earlier builds. Never touch actual
# read timestamps >0, incoming messages, or group messages. On the next valid
# positive receipt the usual status transition can advance them again.
archive="src/archive.rs"
change(archive,
       "        favorites::adopt_local_marks(&connection)?;",
       '''        // Apaajalahh: history-sync once treated zero read_timestamp as read.
        // Downgrade *only* exact false positives from direct outgoing chats.
        connection.execute(
            "UPDATE messages SET status = CASE WHEN COALESCE(delivered_at, 0) > 0 THEN 3 ELSE 2 END, read_at = NULL
             WHERE from_me = 1 AND status = 4 AND read_at = 0
               AND (chat LIKE '%@s.whatsapp.net' OR chat LIKE '%@lid')",
            [],
        )?;
        favorites::adopt_local_marks(&connection)?;''')

# Isolate the new branded build from upstream ZapFast's encrypted sessions and
# settings, so neither installation can alter the other's cache/update state.
paths="src/paths.rs"
change(paths,'match Self::of("zapfast") {','match Self::of("apaajalahh") {')
change(paths,'ProjectDirs::from("me", "paolino", name)?','ProjectDirs::from("com", "apaajalahh", name)?')
for name in ["config","state","cache","run"]:
    change(paths, f'fallback.join("zapfast-{name}")',f'fallback.join("apaajalahh-{name}")')
# Disallow importing legacy FastsApp sessions into this newly isolated profile.
change(paths,
       'pub fn adopt_previous_names(&self) -> std::io::Result<()> {\n        for name in ["fastsapp", "fastwhatsapp"] {',
       'pub fn adopt_previous_names(&self) -> std::io::Result<()> {\n        for name in [] as [&str; 0] {')

# Make default UI more legible at high-resolution desktop sizes; no web engine.
settings="src/settings.rs"
change(settings,"            zoom: 1.0,","            zoom: 1.12,")
change(settings,"            sidebar_width: 320.0,","            sidebar_width: 352.0,")
# The binary is customized; never auto-follow unmodified upstream updates.
change(settings,"            check_for_updates: true,","            check_for_updates: false,")

main="src/main.rs"
change(main,'if demo { "ZapFast Demo" } else { "ZapFast" }','if demo { "Apaajalahh Demo" } else { "Apaajalahh" }')
change(main,'std::env::var("FLATPAK_ID").unwrap_or_else(|_| "zapfast".to_owned())',
       'std::env::var("FLATPAK_ID").unwrap_or_else(|_| "apaajalahh".to_owned())')
change(main,'                "ZapFast",\n','                "Apaajalahh",\n', count=1)
app="src/app.rs"
change(app,'        id: "zapfast",\n        title: "ZapFast".into(),',
       '        id: "apaajalahh",\n        title: "Apaajalahh".into(),')
change(app,'MenuItem::action(TRAY_SHOW, "Show or hide ZapFast")',
       'MenuItem::action(TRAY_SHOW, "Show or hide Apaajalahh")')
change(app,'MenuItem::action(TRAY_LOCK, "Lock ZapFast")',
       'MenuItem::action(TRAY_LOCK, "Lock Apaajalahh")')

# A small visual upgrade with shared semantic tokens, not hundreds of layout
# rewrites. All WCAG text contrast helpers remain upstream and theme-aware.
theme="src/theme.rs"
for old, new in {
"window: Color32::from_rgb(0x0b, 0x14, 0x1a),":"window: Color32::from_rgb(0x0d, 0x16, 0x1d),",
"panel: Color32::from_rgb(0x11, 0x1b, 0x21),":"panel: Color32::from_rgb(0x15, 0x21, 0x2b),",
"surface: Color32::from_rgb(0x20, 0x2c, 0x33),":"surface: Color32::from_rgb(0x20, 0x30, 0x3b),",
"surface_hover: Color32::from_rgb(0x2a, 0x39, 0x42),":"surface_hover: Color32::from_rgb(0x2a, 0x3d, 0x49),",
"surface_active: Color32::from_rgb(0x35, 0x44, 0x4d),":"surface_active: Color32::from_rgb(0x35, 0x50, 0x59),",
"accent: Color32::from_rgb(0x00, 0xa8, 0x84),":"accent: Color32::from_rgb(0x0c, 0xb9, 0x97),",
"accent_hover: Color32::from_rgb(0x06, 0xcf, 0x9c),":"accent_hover: Color32::from_rgb(0x25, 0xd5, 0xb1),",
"chat: Color32::from_rgb(0x0b, 0x14, 0x1a),":"chat: Color32::from_rgb(0x0d, 0x16, 0x1d),",
"bubble_in: Color32::from_rgb(0x20, 0x2c, 0x33),":"bubble_in: Color32::from_rgb(0x20, 0x30, 0x3b),",
"bubble_out: Color32::from_rgb(0x00, 0x5c, 0x4b),":"bubble_out: Color32::from_rgb(0x06, 0x58, 0x48),",
}.items():
    change(theme,old,new)

# A focused regression test, run on Windows CI before publishing a binary.
test = r'''
#[cfg(test)]
mod apaajalahh_receipt_regression {
    use super::*;
    use wa::web_message_info::Status;

    #[test]
    fn zero_history_read_timestamp_must_not_turn_ticks_blue() {
        let parse = |read_time: i64| {
            parse_conversation(wa::Conversation {
                id: "628123456789@s.whatsapp.net".into(),
                messages: vec![wa::HistorySyncMsg {
                    message: MessageField::some(wa::WebMessageInfo {
                        key: MessageField::some(wa::MessageKey {
                            id: Some("apaajalahh-regression".into()),
                            from_me: Some(true),
                            ..Default::default()
                        }),
                        message: MessageField::some(wa::Message {
                            conversation: Some("Test".into()),
                            ..Default::default()
                        }),
                        message_timestamp: Some(100),
                        status: Some(Status::DELIVERY_ACK),
                        user_receipt: vec![wa::UserReceipt {
                            user_jid: "628123456789@s.whatsapp.net".into(),
                            receipt_timestamp: Some(99),
                            read_timestamp: Some(read_time),
                            ..Default::default()
                        }],
                        ..Default::default()
                    }),
                    ..Default::default()
                }],
                ..Default::default()
            }).messages.remove(0).status
        };
        assert_eq!(parse(0), Delivery::Delivered, "unknown read timestamp is not read");
        assert_eq!(parse(123), Delivery::Read, "positive read timestamp is read");
    }
}
'''
with (root / worker).open("a", encoding="utf-8") as file:
    file.write("\n" + test)
print("Appended targeted receipt regression test")
