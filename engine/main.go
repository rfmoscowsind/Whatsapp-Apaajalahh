package main

import (
 "bufio"
 "context"
 "database/sql"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "mime"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "sync/atomic"
 "time"

 "github.com/skip2/go-qrcode"
 "go.mau.fi/whatsmeow"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 waLog "go.mau.fi/whatsmeow/util/log"
 "google.golang.org/protobuf/proto"
 _ "modernc.org/sqlite"
)

const maxUploadBytes int64 = 64 << 20 // avoid large allocations in the initial version
const maxMessageLength = 16000

type Command struct {
 Action string
 Chat string
 Text string
 Path string
 ID string
}

type App struct {
 mu sync.Mutex
 db *sql.DB
 auth *sqlstore.Container
 client *whatsmeow.Client
 connecting atomic.Bool
}

func (a *App) emit(typ string, fields map[string]any) {
 if fields == nil { fields = make(map[string]any) }
 fields["type"] = typ
 a.mu.Lock()
 defer a.mu.Unlock()
 _ = json.NewEncoder(os.Stdout).Encode(fields)
}
func (a *App) status(state, message string) {
 a.emit("status", map[string]any{"state":state, "message":message})
}
func (a *App) report(err error) {
 if err != nil { a.emit("error", map[string]any{"message":err.Error()}) }
}

func initStorage(dir string) (*sql.DB,*sqlstore.Container,*whatsmeow.Client,error) {
 cache,err := sql.Open("sqlite",filepath.Join(dir,"cache.db"))
 if err != nil { return nil,nil,nil,err }
 cache.SetMaxOpenConns(1)
 _,err = cache.Exec("PRAGMA journal_mode=WAL;")
 if err == nil {
  _,err = cache.Exec("CREATE TABLE IF NOT EXISTS chats(chat TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', last_text TEXT NOT NULL DEFAULT '', last_ts INTEGER NOT NULL DEFAULT 0)")
 }
 if err == nil {
  _,err = cache.Exec("CREATE TABLE IF NOT EXISTS messages(chat TEXT NOT NULL, id TEXT NOT NULL, body TEXT NOT NULL, sender TEXT NOT NULL DEFAULT '', mine INTEGER NOT NULL DEFAULT 0, ts INTEGER NOT NULL DEFAULT 0, media_kind TEXT NOT NULL DEFAULT '', filename TEXT NOT NULL DEFAULT '', protobuf BLOB, view_once INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(chat,id))")
 }
 if err != nil { _=cache.Close(); return nil,nil,nil,err }
 authDB,err := sql.Open("sqlite",filepath.Join(dir,"session.db"))
 if err != nil { _=cache.Close(); return nil,nil,nil,err }
 authDB.SetMaxOpenConns(1)
 if _,err=authDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
  _=cache.Close(); _=authDB.Close(); return nil,nil,nil,err
 }
 container := sqlstore.NewWithDB(authDB,"sqlite3",waLog.Noop)
 if err = container.Upgrade(context.Background()); err != nil {
  _=cache.Close(); _=container.Close(); return nil,nil,nil,err
 }
 device,err := container.GetFirstDevice(context.Background())
 if err != nil { _=cache.Close(); _=container.Close(); return nil,nil,nil,err }
 cli := whatsmeow.NewClient(device,waLog.Noop)
 return cache,container,cli,nil
}

func displayName(chat string) string {
 parts:=strings.SplitN(chat,"@",2)
 if len(parts)==0 || parts[0]=="" { return chat }
 return parts[0]
}

func (a *App) upsertChat(chat,name,preview string,ts int64) error {
 if chat=="" {return nil}
 if name=="" {name=displayName(chat)}
 _,err:=a.db.Exec(
  "INSERT INTO chats(chat,name,last_text,last_ts) VALUES(?,?,?,?) "+
  "ON CONFLICT(chat) DO UPDATE SET "+
  "name=CASE WHEN excluded.name!=chats.chat AND excluded.name!='' THEN excluded.name ELSE chats.name END,"+
  "last_text=CASE WHEN excluded.last_ts>=chats.last_ts AND excluded.last_text!='' THEN excluded.last_text ELSE chats.last_text END,"+
  "last_ts=MAX(chats.last_ts,excluded.last_ts)",
  chat,name,preview,ts)
 return err
}

func describeMessage(msg *waE2E.Message) (string,string,string) {
 if msg==nil{return "","",""}
 if t:=msg.GetConversation(); t!="" {return t,"",""}
 if t:=msg.GetExtendedTextMessage().GetText(); t!="" {return t,"",""}
 if v:=msg.GetDocumentMessage();v!=nil {
  return firstText(v.GetCaption(),"Document: "+v.GetFileName()),"document",v.GetFileName()
 }
 if v:=msg.GetImageMessage();v!=nil {return firstText(v.GetCaption(),"Image"),"image","image.jpg"}
 if v:=msg.GetVideoMessage();v!=nil {return firstText(v.GetCaption(),"Video"),"video","video.mp4"}
 if v:=msg.GetAudioMessage();v!=nil {return "Audio message","audio","audio.ogg"}
 if v:=msg.GetStickerMessage();v!=nil {return "Sticker","sticker","sticker.webp"}
 if msg.GetReactionMessage()!=nil {return "Reaction","",""}
 return "Unsupported message","",""
}
func firstText(value, fallback string) string {if value!="" {return value};return fallback}
func isConversation(chat string) bool {
 return strings.HasSuffix(chat,"@s.whatsapp.net") || strings.HasSuffix(chat,"@lid") || strings.HasSuffix(chat,"@g.us")
}
func (a *App) saveMessage(evt *events.Message) {
 if evt==nil || evt.Message==nil || evt.Info.ID=="" {return}
 chat:=evt.Info.Chat.String()
 if !isConversation(chat) {return}
 text,kind,filename:=describeMessage(evt.Message)
 if text=="" {return}
 if len(text)>maxMessageLength {text=text[:maxMessageLength]}
 mine:=0
 if evt.Info.IsFromMe {mine=1}
 name:=""
 if !evt.Info.IsGroup && !evt.Info.IsFromMe {name=evt.Info.PushName}
 if err:=a.upsertChat(chat,name,text,evt.Info.Timestamp.Unix());err!=nil{a.report(err);return}
 var raw []byte
 if !evt.IsViewOnce {raw,_=proto.Marshal(evt.Message)}
 viewOnce:=0
 if evt.IsViewOnce {viewOnce=1;kind="";filename="";text="View-once message (not cached)"}
 _,err:=a.db.Exec("INSERT INTO messages(chat,id,body,sender,mine,ts,media_kind,filename,protobuf,view_once) VALUES(?,?,?,?,?,?,?,?,?,?) "+
  "ON CONFLICT(chat,id) DO UPDATE SET body=excluded.body,sender=excluded.sender,mine=excluded.mine,ts=excluded.ts,media_kind=excluded.media_kind,filename=excluded.filename,protobuf=excluded.protobuf,view_once=excluded.view_once",
  chat,string(evt.Info.ID),text,evt.Info.Sender.String(),mine,evt.Info.Timestamp.Unix(),kind,filename,raw,viewOnce)
 if err!=nil {a.report(err)}
}

func (a *App) emitChats() {
 rows,err:=a.db.Query("SELECT chat,name,last_text,last_ts FROM chats ORDER BY last_ts DESC LIMIT 500")
 if err!=nil {a.report(err);return}
 defer rows.Close()
 result:=make([]map[string]any,0)
 for rows.Next() {
  var chat,name,preview string
  var ts int64
  if rows.Scan(&chat,&name,&preview,&ts)==nil {result=append(result,map[string]any{"chat":chat,"name":name,"preview":preview,"timestamp":ts})}
 }
 a.emit("chats",map[string]any{"items":result})
}

func (a *App) emitMessages(chat string) {
 rows,err:=a.db.Query("SELECT id,body,sender,mine,ts,media_kind,filename FROM "+
  "(SELECT id,body,sender,mine,ts,media_kind,filename FROM messages WHERE chat=? ORDER BY ts DESC,id DESC LIMIT 300) ORDER BY ts ASC,id ASC",chat)
 if err!=nil {a.report(err);return}
 defer rows.Close()
 items:=make([]map[string]any,0)
 for rows.Next() {
  var id,body,sender,kind,filename string
  var mine int
  var ts int64
  if rows.Scan(&id,&body,&sender,&mine,&ts,&kind,&filename)==nil {
   items=append(items,map[string]any{"id":id,"text":body,"sender":sender,"mine":mine==1,"timestamp":ts,"media_kind":kind,"filename":filename})
  }
 }
 a.emit("messages",map[string]any{"chat":chat,"items":items})
}

func (a *App) handleEvent(evt any) {
 switch v:=evt.(type) {
 case *events.Connected:
  a.status("connected","Connected. Messages are syncing in the background.")
  go a.refreshGroups()
 case *events.Disconnected:
  a.status("disconnected","Disconnected. Waiting for reconnection.")
 case *events.LoggedOut:
  a.status("unpaired","Your phone disconnected this device. Login again after restarting.")
 case *events.PairSuccess:
  a.status("pairing","QR accepted. Waiting for secure reconnection.")
 case *events.Message:
  a.saveMessage(v)
  a.emit("refresh",map[string]any{"chat":v.Info.Chat.String()})
 case *events.HistorySync:
  if v.Data==nil{return}
  for _,conv:=range v.Data.GetConversations() {
   jid,err:=types.ParseJID(conv.GetID())
   if err!=nil || !isConversation(jid.String()) {continue}
   if err=a.upsertChat(jid.String(),conv.GetName(),"",int64(conv.GetConversationTimestamp()));err!=nil {continue}
   for _,hist:=range conv.GetMessages() {
    msg:=hist.GetMessage()
    if msg==nil {continue}
    parsed,parseErr:=a.client.ParseWebMessage(jid,msg)
    if parseErr==nil {a.saveMessage(parsed)}
   }
  }
  a.emitChats()
 case *events.JoinedGroup:
  _=a.upsertChat(v.JID.String(),v.Name,"",0)
  a.emitChats()
 case *events.GroupInfo:
  if v.Name!=nil {_=a.upsertChat(v.JID.String(),v.Name.Name,"",0);a.emitChats()}
 case *events.ClientOutdated:
  a.status("error","WhatsApp rejected this client version. Update the engine.")
 case *events.StreamReplaced:
  a.status("error","Another client used this linked session. Restart the app.")
 }
}
func (a *App) refreshGroups() {
 ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
 defer cancel()
 groups,err:=a.client.GetJoinedGroups(ctx)
 if err!=nil{return}
 for _,group:=range groups {
  if group!=nil {_=a.upsertChat(group.JID.String(),group.Name,"",0)}
 }
 a.emitChats()
}

func (a *App) connect() {
 if a.client.IsConnected() {a.status("connected","Already connected.");return}
 if !a.connecting.CompareAndSwap(false,true) {return}
 defer a.connecting.Store(false)
 a.status("connecting","Connecting to WhatsApp...")
 if a.client.Store.ID==nil {
  channel,err:=a.client.GetQRChannel(context.Background())
  if err!=nil {a.report(fmt.Errorf("QR setup: %w",err));a.status("error","Cannot start QR login.");return}
  go func() {
   for item:=range channel {
    if item.Event=="code" {
     png,err:=qrcode.Encode(item.Code,qrcode.Medium,330)
     if err==nil {a.emit("qr",map[string]any{"png":base64.StdEncoding.EncodeToString(png)})}
    } else if item.Event!="success" {
     a.emit("pair_event",map[string]any{"event":item.Event})
    }
   }
  }()
 }
 if err:=a.client.Connect();err!=nil {
  a.report(fmt.Errorf("connection failed: %w",err))
  a.status("error","Connection failed. Check internet and try again.")
 }
}
func (a *App) requireOnline() error {
 if !a.client.IsConnected() || !a.client.IsLoggedIn() {return errors.New("not connected; link your test phone and retry")}
 return nil
}
func (a *App) sendText(cmd Command) error {
 if err:=a.requireOnline();err!=nil{return err}
 text:=strings.TrimSpace(cmd.Text)
 if text=="" {return errors.New("message is empty")}
 if len(text)>maxMessageLength {return errors.New("text too long")}
 jid,err:=types.ParseJID(cmd.Chat)
 if err!=nil || !isConversation(cmd.Chat) {return errors.New("invalid chat ID")}
 ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second)
 defer cancel()
 msg:=&waE2E.Message{Conversation:proto.String(text)}
 response,err:=a.client.SendMessage(ctx,jid,msg)
 if err!=nil{return err}
 a.saveMessage(&events.Message{Info:types.MessageInfo{
  MessageSource:types.MessageSource{Chat:jid,Sender:jid,IsFromMe:true},
  ID:response.ID,Timestamp:response.Timestamp,
 },Message:msg})
 a.emit("refresh",map[string]any{"chat":cmd.Chat})
 a.emit("sent",map[string]any{"chat":cmd.Chat,"id":string(response.ID)})
 return nil
}
func (a *App) sendFile(cmd Command) error {
 if err:=a.requireOnline();err!=nil{return err}
 jid,err:=types.ParseJID(cmd.Chat)
 if err!=nil || !isConversation(cmd.Chat){return errors.New("invalid chat ID")}
 stat,err:=os.Stat(cmd.Path)
 if err!=nil {return err}
 if !stat.Mode().IsRegular() || stat.Size()>maxUploadBytes {return fmt.Errorf("only regular files up to %d MiB are supported",maxUploadBytes>>20)}
 data,err:=os.ReadFile(cmd.Path)
 if err!=nil{return err}
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Minute)
 defer cancel()
 a.status("uploading","Uploading document...")
 response,err:=a.client.Upload(ctx,data,whatsmeow.MediaDocument)
 if err!=nil{return err}
 fileName:=filepath.Base(cmd.Path)
 mediaType:=mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName)))
 if mediaType==""{mediaType="application/octet-stream"}
 doc:=&waE2E.DocumentMessage{
  URL:&response.URL,DirectPath:&response.DirectPath,MediaKey:response.MediaKey,
  FileSHA256:response.FileSHA256,FileEncSHA256:response.FileEncSHA256,
  FileLength:&response.FileLength,Mimetype:proto.String(mediaType),FileName:proto.String(fileName),
 }
 msg:=&waE2E.Message{DocumentMessage:doc}
 sent,err:=a.client.SendMessage(ctx,jid,msg)
 if err!=nil{return err}
 a.saveMessage(&events.Message{Info:types.MessageInfo{
  MessageSource:types.MessageSource{Chat:jid,Sender:jid,IsFromMe:true},
  ID:sent.ID,Timestamp:sent.Timestamp,
 },Message:msg})
 a.emit("refresh",map[string]any{"chat":cmd.Chat})
 a.emit("sent",map[string]any{"chat":cmd.Chat,"id":string(sent.ID)})
 a.status("connected","Document sent.")
 return nil
}
func (a *App) download(cmd Command) error {
 if err:=a.requireOnline();err!=nil{return err}
 if !isConversation(cmd.Chat) || cmd.ID=="" || cmd.Path=="" {return errors.New("invalid download request")}
 var raw []byte
 var once int
 err:=a.db.QueryRow("SELECT protobuf,view_once FROM messages WHERE chat=? AND id=?",cmd.Chat,cmd.ID).Scan(&raw,&once)
 if err!=nil {return err}
 if once==1 || len(raw)==0 {return errors.New("view-once or unavailable media cannot be downloaded")}
 var msg waE2E.Message
 if err=proto.Unmarshal(raw,&msg);err!=nil{return err}
 var downloadable whatsmeow.DownloadableMessage
 switch {
 case msg.GetDocumentMessage()!=nil: downloadable=msg.GetDocumentMessage()
 case msg.GetImageMessage()!=nil: downloadable=msg.GetImageMessage()
 case msg.GetVideoMessage()!=nil: downloadable=msg.GetVideoMessage()
 case msg.GetAudioMessage()!=nil: downloadable=msg.GetAudioMessage()
 case msg.GetStickerMessage()!=nil: downloadable=msg.GetStickerMessage()
 default: return errors.New("message does not contain supported media")
 }
 if _,err=os.Stat(cmd.Path);err==nil {return errors.New("file already exists: choose a different destination")}
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Minute)
 defer cancel()
 a.status("downloading","Downloading and decrypting media...")
 bytes,err:=a.client.Download(ctx,downloadable)
 if err!=nil{return err}
 dir:=filepath.Dir(cmd.Path)
 f,err:=os.CreateTemp(dir,".wanative-*")
 if err!=nil{return err}
 defer os.Remove(f.Name())
 if _,err=f.Write(bytes);err!=nil {_=f.Close();return err}
 if err=f.Close();err!=nil{return err}
 if err=os.Rename(f.Name(),cmd.Path);err!=nil{return err}
 a.emit("downloaded",map[string]any{"path":cmd.Path})
 a.status("connected","File downloaded.")
 return nil
}
func (a *App) handleCommand(cmd Command) {
 var err error
 switch cmd.Action {
 case "chats":a.emitChats()
 case "messages":if isConversation(cmd.Chat){a.emitMessages(cmd.Chat)}
 case "connect":a.connect()
 case "send_text":err=a.sendText(cmd)
 case "send_file":err=a.sendFile(cmd)
 case "download":err=a.download(cmd)
 case "logout":
  ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second)
  err=a.client.Logout(ctx)
  cancel()
  if err==nil {
   _,_=a.db.Exec("DELETE FROM messages")
   _,_=a.db.Exec("DELETE FROM chats")
   a.emitChats()
   a.status("unpaired","Logged out. Restart application to pair again.")
  }
 default:err=fmt.Errorf("unsupported operation")
 }
 if err!=nil {a.report(err);if cmd.Action=="send_file" || cmd.Action=="download" {a.status("connected","Transfer failed.")}}
}

func run(stdin io.Reader) error {
 cfg,err:=os.UserConfigDir()
 if err!=nil{return err}
 dir:=filepath.Join(cfg,"WA-Native-Lite")
 if err=os.MkdirAll(dir,0700);err!=nil{return err}
 db,auth,client,err:=initStorage(dir)
 if err!=nil{return err}
 defer db.Close()
 defer auth.Close()
 defer client.Disconnect()
 app:=&App{db:db,auth:auth,client:client}
 client.AddEventHandler(app.handleEvent)
 app.emitChats()
 if client.Store.ID!=nil {
  go app.connect()
 } else {app.status("unpaired","Link a test account with the QR button.")}
 scanner:=bufio.NewScanner(stdin)
 scanner.Buffer(make([]byte,4096),1<<20)
 for scanner.Scan() {
  var cmd Command
  if err=json.Unmarshal(scanner.Bytes(),&cmd);err!=nil {app.report(err);continue}
  if cmd.Action=="chats" || cmd.Action=="messages" {
   app.handleCommand(cmd)
  } else {go app.handleCommand(cmd)}
 }
 return scanner.Err()
}
func main() {
 if err:=run(os.Stdin);err!=nil {
  _=json.NewEncoder(os.Stdout).Encode(map[string]string{"type":"fatal","message":err.Error()})
  fmt.Fprintln(os.Stderr,"engine startup failed:",err)
  os.Exit(1)
 }
}
