package main

import (
 "context"
 "testing"
 "time"

 "go.mau.fi/whatsmeow/proto/waE2E"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
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

func TestSessionDatabaseInitializationAndCache(t *testing.T) {
 db, auth, client, err := initStorage(t.TempDir())
 if err!=nil {t.Fatalf("initStorage: %v",err)}
 defer db.Close()
 defer auth.Close()
 defer client.Disconnect()
 if client.Store.ID!=nil {t.Fatal("new session unexpectedly paired")}
 var tables int
 if err=db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='messages'").Scan(&tables);err!=nil || tables!=1 {
  t.Fatalf("message table missing: %v",err)
 }
 a:=&App{db:db,auth:auth,client:client}
 jid,err:=types.ParseJID("628123456789@s.whatsapp.net")
 if err!=nil {t.Fatal(err)}
 a.saveMessage(&events.Message{Info:types.MessageInfo{
  MessageSource:types.MessageSource{Chat:jid,Sender:jid,IsFromMe:false},
  ID:"test-message-1",Timestamp:time.Unix(1700000000,0),
 },Message:&waE2E.Message{Conversation:proto.String("cached hello")}})
 var got string
 if err=db.QueryRowContext(context.Background(),"SELECT body FROM messages WHERE chat=? AND id=?",jid.String(),"test-message-1").Scan(&got);err!=nil {
  t.Fatalf("read back cached message: %v",err)
 }
 if got!="cached hello" {t.Fatalf("cache mismatch: %q",got)}
 var chatCount int
 if err=db.QueryRow("SELECT COUNT(*) FROM chats").Scan(&chatCount);err!=nil || chatCount!=1 {
  t.Fatalf("chat wasn't indexed: %d, %v",chatCount,err)
 }
}
