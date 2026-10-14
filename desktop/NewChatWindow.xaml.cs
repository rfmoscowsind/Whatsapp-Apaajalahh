using System.Windows;

namespace WANativeLite;
public partial class NewChatWindow : Window
{
    public NewChatWindow() => InitializeComponent();
    public string PhoneNumber => PhoneBox.Text.Trim();
    private void Save_Click(object sender, RoutedEventArgs e) { DialogResult = true; Close(); }
    private void Cancel_Click(object sender, RoutedEventArgs e) { DialogResult = false; Close(); }
}
