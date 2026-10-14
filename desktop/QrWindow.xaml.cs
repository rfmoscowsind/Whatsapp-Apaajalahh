using System.IO;
using System.Windows;
using System.Windows.Media.Imaging;

namespace WANativeLite;
public partial class QrWindow : Window
{
    public QrWindow() => InitializeComponent();
    public void UpdateQr(string base64)
    {
        try
        {
            using var stream = new MemoryStream(Convert.FromBase64String(base64));
            var image = new BitmapImage();
            image.BeginInit();
            image.CacheOption = BitmapCacheOption.OnLoad;
            image.StreamSource = stream;
            image.EndInit();
            image.Freeze();
            QrImage.Source = image;
        }
        catch (FormatException) { /* ignore malformed QR */ }
    }
}
