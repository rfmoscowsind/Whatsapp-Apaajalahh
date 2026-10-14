using System.Collections.ObjectModel;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using Microsoft.Win32;

namespace WANativeLite;

public partial class MainWindow : Window
{
    private sealed class DemoChat
    {
        public string Name { get; }
        public ObservableCollection<string> Messages { get; } = new();
        public DemoChat(string name, params string[] messages)
        {
            Name = name;
            foreach (var message in messages) Messages.Add(message);
        }
    }

    private readonly List<DemoChat> _chats =
    [
        new("Team Project", "Welcome to the native desktop prototype!", "This is a local demo, not a WhatsApp conversation."),
        new("Testing Files", "Use Attach to select a local file."),
        new("UI Performance", "No embedded Chromium or WebView here.")
    ];

    private DemoChat? _selected;

    public MainWindow()
    {
        InitializeComponent();
        RefreshChats();
        ChatsList.SelectedIndex = 0;
    }

    private void RefreshChats()
    {
        var query = SearchBox.Text?.Trim() ?? "";
        var selected = _selected;
        ChatsList.ItemsSource = _chats
            .Where(chat => chat.Name.Contains(query, StringComparison.OrdinalIgnoreCase))
            .ToList();
        if (selected != null) ChatsList.SelectedItem = selected;
    }

    private void SearchBox_TextChanged(object sender, TextChangedEventArgs e) => RefreshChats();

    private void ChatsList_SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        _selected = ChatsList.SelectedItem as DemoChat;
        ChatTitle.Text = _selected?.Name ?? "Select a conversation";
        RenderMessages();
    }

    private void RenderMessages()
    {
        MessagePanel.Children.Clear();
        if (_selected == null) return;
        foreach (var message in _selected.Messages)
        {
            MessagePanel.Children.Add(new Border
            {
                Background = new SolidColorBrush(Color.FromRgb(32, 44, 51)),
                CornerRadius = new CornerRadius(8),
                Padding = new Thickness(13, 10, 13, 10),
                Margin = new Thickness(0, 0, 0, 10),
                MaxWidth = 560,
                HorizontalAlignment = HorizontalAlignment.Left,
                Child = new TextBlock
                {
                    Text = message,
                    TextWrapping = TextWrapping.Wrap,
                    Foreground = Brushes.White,
                    FontSize = 14
                }
            });
        }
        MessageScroll.ScrollToEnd();
    }

    private void Send_Click(object sender, RoutedEventArgs e) => SendDemoMessage();

    private void MessageInput_KeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter) { SendDemoMessage(); e.Handled = true; }
    }

    private void SendDemoMessage()
    {
        var text = MessageInput.Text.Trim();
        if (string.IsNullOrEmpty(text) || _selected == null) return;
        _selected.Messages.Add("You (demo): " + text);
        MessageInput.Clear();
        RenderMessages();
    }

    private void Attach_Click(object sender, RoutedEventArgs e)
    {
        if (_selected == null) return;
        var dialog = new OpenFileDialog
        {
            Title = "Choose a file (demo only)",
            Filter = "All files|*.*"
        };
        if (dialog.ShowDialog(this) != true) return;
        _selected.Messages.Add("Selected file (not uploaded): " + System.IO.Path.GetFileName(dialog.FileName));
        RenderMessages();
    }
}
