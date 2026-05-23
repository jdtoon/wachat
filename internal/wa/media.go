package wa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

// MediaType is the wachat-local enum for the supported attachment
// kinds. Stays as strings so the SQLite column reads naturally.
const (
	MediaTypeImage    = "image"
	MediaTypeVideo    = "video"
	MediaTypeAudio    = "audio"
	MediaTypeDocument = "document"
	MediaTypeSticker  = "sticker"
)

// MediaInfo is the slice of an inbound message that the wa boundary
// extracts on the way to the store. Caption (if present) lands as the
// message body so existing text-search and rendering keep working;
// ThumbnailJPEG, if non-empty, is written to disk by the handler and
// the path stored in MediaPath.
type MediaInfo struct {
	Type          string
	Caption       string
	ThumbnailJPEG []byte

	// Audio: duration in seconds (0 = unknown).
	DurationSecs uint32
	// Documents + audio: byte size of the underlying file.
	FileSize uint64
	// Documents: server-side filename.
	FileName string
}

// extractMedia inspects a whatsmeow E2E message and returns the media
// info for the message types we currently surface. Empty Type means
// "no recognized attachment" — caller should treat it as text-only.
//
// Currently extracts image / video / audio / document / sticker
// metadata + caption + embedded thumbnail (for image and video).
// Full-resolution download is a separate path (see WA.DownloadImage).
func extractMedia(m *waE2E.Message) MediaInfo {
	if m == nil {
		return MediaInfo{}
	}
	if im := m.GetImageMessage(); im != nil {
		return MediaInfo{
			Type:          MediaTypeImage,
			Caption:       im.GetCaption(),
			ThumbnailJPEG: im.GetJPEGThumbnail(),
		}
	}
	if vm := m.GetVideoMessage(); vm != nil {
		return MediaInfo{
			Type:          MediaTypeVideo,
			Caption:       vm.GetCaption(),
			ThumbnailJPEG: vm.GetJPEGThumbnail(),
		}
	}
	if am := m.GetAudioMessage(); am != nil {
		return MediaInfo{
			Type:         MediaTypeAudio,
			DurationSecs: am.GetSeconds(),
			FileSize:     am.GetFileLength(),
		}
	}
	if dm := m.GetDocumentMessage(); dm != nil {
		name := dm.GetFileName()
		if name == "" {
			name = dm.GetTitle()
		}
		return MediaInfo{
			Type:          MediaTypeDocument,
			Caption:       name,
			FileName:      name,
			FileSize:      dm.GetFileLength(),
			ThumbnailJPEG: dm.GetJPEGThumbnail(),
		}
	}
	if sm := m.GetStickerMessage(); sm != nil {
		return MediaInfo{Type: MediaTypeSticker}
	}
	return MediaInfo{}
}

// FormatDuration renders an audio length as M:SS for the bubble pill.
// Public so the UI layer can format DurationSecs in MessageEvent.
func FormatDuration(secs uint32) string {
	if secs == 0 {
		return ""
	}
	m := secs / 60
	s := secs % 60
	// Manual format — keeps the wa package without an fmt allocation
	// per render frame. Caller can switch to fmt.Sprintf if locale
	// matters later.
	return itoa(int(m)) + ":" + zeropad2(int(s))
}

// FormatBytes renders a file size as B / KB / MB / GB. Same
// rationale as FormatDuration.
func FormatBytes(b uint64) string {
	const KB = 1024
	const MB = KB * 1024
	const GB = MB * 1024
	switch {
	case b == 0:
		return ""
	case b < KB:
		return itoa(int(b)) + " B"
	case b < MB:
		return itoa(int(b/KB)) + " KB"
	case b < GB:
		// One decimal place.
		whole := int(b / MB)
		frac := int((b % MB) * 10 / MB)
		return itoa(whole) + "." + itoa(frac) + " MB"
	default:
		whole := int(b / GB)
		frac := int((b % GB) * 10 / GB)
		return itoa(whole) + "." + itoa(frac) + " GB"
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func zeropad2(i int) string {
	if i < 10 {
		return "0" + itoa(i)
	}
	return itoa(i)
}

// MediaDir is the on-disk directory for media files. Set once at
// startup via SetMediaDir; defaults to "media/" relative to cwd.
var mediaDir = "media"

// SetMediaDir overrides the default media root. Call once before
// wa.New + AddEventHandler.
func SetMediaDir(d string) { mediaDir = d }

// MediaDir returns the current media-root path.
func MediaDir() string { return mediaDir }

// writeThumbnail saves bytes to a stable path keyed on waID. Returns
// the path that should be stored in messages.media_path. Empty input
// is a no-op (returns "").
func writeThumbnail(waID string, b []byte) (string, error) {
	if len(b) == 0 || waID == "" {
		return "", nil
	}
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("wa.writeThumbnail: mkdir: %w", err)
	}
	// .jpg because whatsmeow's JPEGThumbnail is JPEG by definition.
	path := filepath.Join(mediaDir, waID+".jpg")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("wa.writeThumbnail: write: %w", err)
	}
	return path, nil
}

// DownloadImage fetches the full-resolution image for a message and
// writes it to disk under MediaDir(). Returns the file path.
//
// Used by the future "click thumbnail to open" path; not yet wired
// into the UI but exposed so tests and tools can exercise it.
func (c *Client) DownloadImage(ctx context.Context, m *waE2E.Message, waID string) (string, error) {
	if c == nil || c.wm == nil {
		return "", fmt.Errorf("wa.DownloadImage: client is nil")
	}
	im := m.GetImageMessage()
	if im == nil {
		return "", fmt.Errorf("wa.DownloadImage: not an image message")
	}
	bytes, err := c.wm.Download(ctx, im)
	if err != nil {
		return "", fmt.Errorf("wa.DownloadImage: %w", err)
	}
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("wa.DownloadImage: mkdir: %w", err)
	}
	ext := imageExt(im.GetMimetype())
	path := filepath.Join(mediaDir, waID+ext)
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return "", fmt.Errorf("wa.DownloadImage: write: %w", err)
	}
	return path, nil
}

// imageExt picks a sensible file extension for an image mime.
func imageExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// ensureDownloadable keeps the unused-import linter happy until the
// download flow is wired into the UI. Kept here so callers exist for
// every imported symbol while the broader media UX lands.
var _ = whatsmeow.DownloadableMessage(nil)
