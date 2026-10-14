using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Input;
using System.Windows.Media;
using Microsoft.Win32;

namespace WANativeLite;

public sealed class ChatRow
{
    public string Chat { get; init; } = "";
    public string Name { get; init; } = "";
    public string Preview { get; init; } = "";
}
public sealed class MessageRow
{
    private static readonly Brush IncomingBrush = new SolidColorBrush(Color.FromRgb(32, 44, 51));
    private static readonly Brush OutgoingBrush = new SolidColorBrush(Color.FromRgb(0, 92, 75));
    public string Id { get; init; } = "";
    public string Chat { get; init; } = "";
    public string Text { get; init; } = "";
    public string Filename { get; init; } = "";
    public string MediaKind { get; init; } = "";
    public string SenderLabel { get; init; } = "";
    public string TimeLabel { get; init; } = "";
    public bool Mine { get; init; }
    public Brush BubbleBrush => Mine ? OutgoingBrush : IncomingBrush;
    public HorizontalAlignment Placement => Mine ? HorizontalAlignment.Right : HorizontalAlignment.Left;
    public Visibility DownloadVisibility => MediaKind.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
    public string DownloadLabel => "↓ Save " + (Filename.Length > 0 ? Filename : MediaKind);
}

public partial class MainWindow : Window
{
    private readonly ObservableCollection<ChatRow> _chats = new();
    private readonly ObservableCollection<MessageRow> _messages = new();
    private readonly object _writeLock = new();
    private Process? _engine;
    private QrWindow? _qrWindow;
    private ChatRow? _selectedChat;
    private bool _connected;
    private bool _closing;

    public MainWindow()
    {
        InitializeComponent();
        ChatsList.ItemsSource = _chats;
        MessagesList.ItemsSource = _messages;
        CollectionViewSource.GetDefaultView(_chats).Filter = FilterChat;
    }

    private static string Get(JsonElement node, string key, string fallback = "")
    {
        return node.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.String
            ? (v.GetString() ?? fallback) : fallback;
    }
    private static long GetLong(JsonElement node, string key)
    {
        return node.TryGetProperty(key, out var v) && v.TryGetInt64(out var value) ? value : 0;
    }
    private static bool GetBool(JsonElement node, string key)
    {
        return node.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.True;
    }

    private bool FilterChat(object obj)
    {
        if (obj is not ChatRow row) return false;
        var query = SearchBox.Text.Trim();
        return query.Length == 0 || row.Name.Contains(query, StringComparison.OrdinalIgnoreCase)
            || row.Chat.Contains(query, StringComparison.OrdinalIgnoreCase);
    }
    private void SearchBox_TextChanged(object sender, TextChangedEventArgs e)
        => CollectionViewSource.GetDefaultView(_chats)?.Refresh();

    private void Window_Loaded(object sender, RoutedEventArgs e) => StartEngine();

    private void StartEngine()
    {
        var executable = Path.Combine(AppContext.BaseDirectory, "engine", "wa-engine.exe");
        if (!File.Exists(executable))
        {
            StatusText.Text = "Engine is missing. Extract the entire release ZIP before running.";
            LoginButton.IsEnabled = false;
            return;
        }
        try
        {
            _engine = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = executable,
                    WorkingDirectory = AppContext.BaseDirectory,
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    RedirectStandardInput = true,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    StandardInputEncoding = new UTF8Encoding(false),
                    StandardOutputEncoding = new UTF8Encoding(false)
                },
                EnableRaisingEvents = true
            };
            _engine.Start();
            _engine.BeginErrorReadLine();
            _ = Task.Run(ReadEngineAsync);
        }
        catch (Exception ex)
        {
            StatusText.Text = "Could not start engine: " + ex.Message;
            LoginButton.IsEnabled = false;
        }
    }

    private async Task ReadEngineAsync()
    {
        var engine = _engine;
        if (engine == null) return;
        try
        {
            while (await engine.StandardOutput.ReadLineAsync() is { } line)
            {
                using var json = JsonDocument.Parse(line);
                var payload = json.RootElement.Clone();
                await Dispatcher.InvokeAsync(() =>
                {
                    if (!_closing) HandleEngineEvent(payload);
                });
            }
            await Dispatcher.InvokeAsync(() =>
            {
                if (!_closing) StatusText.Text = "Messaging engine has stopped. Restart the application.";
            });
        }
        catch (Exception ex)
        {
            await Dispatcher.InvokeAsync(() =>
            {
                if (!_closing) StatusText.Text = "Engine communication error: " + ex.Message;
            });
        }
    }

    private void SendCommand(object payload)
    {
        try
        {
            lock (_writeLock)
            {
                if (_engine == null || _engine.HasExited)
                {
                    StatusText.Text = "Engine not running. Restart the application.";
                    return;
                }
                _engine.StandardInput.WriteLine(JsonSerializer.Serialize(payload));
                _engine.StandardInput.Flush();
            }
        }
        catch (Exception ex) { StatusText.Text = "IPC error: " + ex.Message; }
    }

    private void HandleEngineEvent(JsonElement payload)
    {
        var typ = Get(payload, "type");
        switch (typ)
        {
            case "status":
                SetStatus(Get(payload, "state"), Get(payload, "message"));
                break;
            case "error":
            case "fatal":
                StatusText.Text = Get(payload, "message", "Unknown engine error.");
                break;
            case "qr":
                ShowQr(Get(payload, "png"));
                break;
            case "chats":
                ApplyChats(payload.GetProperty("items"));
                break;
            case "messages":
                ApplyMessages(Get(payload, "chat"), payload.GetProperty("items"));
                break;
            case "refresh":
                SendCommand(new { action = "chats" });
                if (_selectedChat?.Chat == Get(payload, "chat"))
                    SendCommand(new { action = "messages", chat = _selectedChat.Chat });
                break;
            case "downloaded":
                StatusText.Text = "Saved: " + Get(payload, "path");
                break;
            case "pair_event":
                if (Get(payload, "event") == "timeout")
                    StatusText.Text = "QR expired. Click Link phone to try again.";
                break;
            case "sent":
                StatusText.Text = "Message accepted by WhatsApp server.";
                break;
        }
    }

    private void SetStatus(string state, string message)
    {
        StatusText.Text = message;
        _connected = state is "connected" or "uploading" or "downloading";
        LogoutButton.Visibility = _connected ? Visibility.Visible : Visibility.Collapsed;
        LoginButton.Visibility = _connected ? Visibility.Collapsed : Visibility.Visible;
        if (_connected && _qrWindow != null)
        {
            _qrWindow.Close();
            _qrWindow = null;
        }
    }

    private void ShowQr(string encodedPng)
    {
        if (encodedPng.Length == 0) return;
        _qrWindow ??= new QrWindow { Owner = this };
        if (!_qrWindow.IsVisible)
        {
            _qrWindow.Closed += (_, _) => _qrWindow = null;
            _qrWindow.Show();
        }
        _qrWindow.UpdateQr(encodedPng);
        StatusText.Text = "Scan QR from WhatsApp > Linked devices.";
    }

    private void ApplyChats(JsonElement items)
    {
        var current = _selectedChat?.Chat;
        var temp = new List<ChatRow>();
        foreach (var item in items.EnumerateArray())
            temp.Add(new ChatRow
            {
                Chat = Get(item, "chat"),
                Name = Get(item, "name"),
                Preview = Get(item, "preview", "No messages yet")
            });
        // Keep a newly composed chat in the list until its first message has been sent.
        if (_selectedChat != null && !temp.Any(x => x.Chat == _selectedChat.Chat))
            temp.Insert(0, _selectedChat);
        _chats.Clear();
        foreach (var chat in temp) _chats.Add(chat);
        var selected = _chats.FirstOrDefault(x => x.Chat == current) ?? _chats.FirstOrDefault();
        if (selected != null) ChatsList.SelectedItem = selected;
        else { _selectedChat = null; _messages.Clear(); ChatTitle.Text = "Choose a conversation"; }
    }

    private void ApplyMessages(string chat, JsonElement items)
    {
        if (_selectedChat?.Chat != chat) return;
        _messages.Clear();
        foreach (var item in items.EnumerateArray())
        {
            var mine = GetBool(item, "mine");
            var ts = GetLong(item, "timestamp");
            var time = ts > 0 ? DateTimeOffset.FromUnixTimeSeconds(ts).ToLocalTime().ToString("dd MMM HH:mm") : "";
            _messages.Add(new MessageRow
            {
                Chat = chat,
                Id = Get(item, "id"),
                Text = Get(item, "text"),
                Filename = Get(item, "filename"),
                MediaKind = Get(item, "media_kind"),
                Mine = mine,
                SenderLabel = mine ? "You" : Get(item, "sender").Split('@')[0],
                TimeLabel = time
            });
        }
        if (_messages.Count > 0) MessagesList.ScrollIntoView(_messages[^1]);
    }

    private void ChatsList_SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        _selectedChat = ChatsList.SelectedItem as ChatRow;
        _messages.Clear();
        ChatTitle.Text = _selectedChat?.Name ?? "Choose a conversation";
        ChatSubtitle.Text = _selectedChat?.Chat ?? "Select a conversation to view cached messages";
        if (_selectedChat != null) SendCommand(new { action = "messages", chat = _selectedChat.Chat });
    }

    private void Login_Click(object sender, RoutedEventArgs e)
    {
        StatusText.Text = "Requesting QR from engine...";
        SendCommand(new { action = "connect" });
    }

    private void Logout_Click(object sender, RoutedEventArgs e)
    {
        var answer = MessageBox.Show(this, "Unlink this test device and clear local cached chats?", "Confirm logout",
            MessageBoxButton.YesNo, MessageBoxImage.Warning);
        if (answer == MessageBoxResult.Yes) SendCommand(new { action = "logout" });
    }

    private void NewChat_Click(object sender, RoutedEventArgs e)
    {
        var dialog = new NewChatWindow { Owner = this };
        if (dialog.ShowDialog() != true) return;
        var digits = Regex.Replace(dialog.PhoneNumber, "[^0-9]", "");
        if (digits.Length < 8 || digits.Length > 16)
        {
            MessageBox.Show(this, "Enter a phone number with country code, e.g. 628xxxxxxxxx.");
            return;
        }
        var jid = digits + "@s.whatsapp.net";
        var chat = _chats.FirstOrDefault(x => x.Chat == jid);
        if (chat == null)
        {
            chat = new ChatRow { Chat = jid, Name = "+" + digits, Preview = "New chat (not yet sent)" };
            _chats.Insert(0, chat);
        }
        ChatsList.SelectedItem = chat;
    }

    private void Send_Click(object sender, RoutedEventArgs e) => SendText();

    private void MessageInput_KeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter && !Keyboard.Modifiers.HasFlag(ModifierKeys.Shift))
        {
            SendText();
            e.Handled = true;
        }
    }

    private void SendText()
    {
        if (!_connected) { StatusText.Text = "Connect WhatsApp before sending."; return; }
        if (_selectedChat == null) { StatusText.Text = "Select a conversation."; return; }
        var text = MessageInput.Text.Trim();
        if (text.Length == 0) return;
        SendCommand(new { action = "send_text", chat = _selectedChat.Chat, text });
        MessageInput.Clear();
    }

    private void Attach_Click(object sender, RoutedEventArgs e)
    {
        if (!_connected || _selectedChat == null) { StatusText.Text = "Connect and select a chat first."; return; }
        var picker = new OpenFileDialog
        {
            Title = "Send a document (up to 64 MiB)",
            Filter = "All files|*.*"
        };
        if (picker.ShowDialog(this) != true) return;
        var info = new FileInfo(picker.FileName);
        if (info.Length > 64L * 1024 * 1024)
        {
            MessageBox.Show(this, "This trial build supports files up to 64 MiB.");
            return;
        }
        SendCommand(new { action = "send_file", chat = _selectedChat.Chat, path = picker.FileName });
    }

    private void Download_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: MessageRow row }) return;
        if (!_connected) { StatusText.Text = "Connect to download an attachment."; return; }
        var suggested = Path.GetFileName(row.Filename);
        foreach (var invalid in Path.GetInvalidFileNameChars()) suggested = suggested.Replace(invalid, '_');
        if (suggested.Length == 0) suggested = "attachment.bin";
        var picker = new SaveFileDialog { FileName = suggested, Title = "Save decrypted attachment", OverwritePrompt = true };
        if (picker.ShowDialog(this) != true) return;
        if (File.Exists(picker.FileName))
        {
            // The engine intentionally refuses to overwrite an existing file.
            MessageBox.Show(this, "Choose a new filename; overwriting existing files is disabled for safety.");
            return;
        }
        SendCommand(new { action = "download", chat = row.Chat, id = row.Id, path = picker.FileName });
    }

    private void Window_Closing(object? sender, CancelEventArgs e)
    {
        _closing = true;
        try
        {
            if (_engine is { HasExited: false })
            {
                _engine.Kill(entireProcessTree: true);
                _engine.WaitForExit(2000);
            }
            _engine?.Dispose();
        }
        catch (Exception) { /* process already gone */ }
    }
}
