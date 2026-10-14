package main

import (
 "testing"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
func TestDescribeText(t *testing.T) {
 text,kind,_:=describeMessage(&waE2E.Message{Conversation:proto.String("hello")})
 if text!="hello" || kind!="" {t.Fatalf("unexpected: %q %q",text,kind)}
}
func TestDescribeDocument(t *testing.T) {
 text,kind,name:=describeMessage(&waE2E.Message{DocumentMessage:&waE2E.DocumentMessage{FileName:proto.String("test.pdf")}})
 if text!="Document: test.pdf" || kind!="document" || name!="test.pdf" {t.Fatalf("unexpected: %q %q %q",text,kind,name)}
}
func TestFilterJID(t *testing.T) {
 if !isConversation("123@s.whatsapp.net") || !isConversation("123@g.us") || isConversation("status@broadcast") {t.Fatal("bad JID filtering")}
}
