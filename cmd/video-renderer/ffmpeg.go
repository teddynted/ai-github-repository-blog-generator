package main

import "strconv"

func itoa(i int) string { return strconv.Itoa(i) }

// dimsFor returns the pixel dimensions for a format aspect ratio. Anything other
// than 16:9 is treated as vertical 9:16 (shorts/tiktok).
func dimsFor(aspect string) (w, h int) {
	if aspect == "16:9" {
		return 1920, 1080
	}
	return 1080, 1920
}

// segmentArgs builds the ffmpeg args that render one scene into an MP4: a solid
// background sized to the format, the scene caption burned in via drawtext (read
// from a file to avoid escaping), and the narration audio. -shortest makes the
// clip exactly as long as the narration.
func segmentArgs(captionFile, narrationMP3, out string, w, h int, fontFile string) []string {
	return []string{
		"-y",
		"-f", "lavfi", "-i", sprintfColor(w, h),
		"-i", narrationMP3,
		"-vf", drawtext(captionFile, fontFile, h),
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k",
		"-shortest",
		out,
	}
}

// concatArgs builds the ffmpeg args that stitch the per-scene MP4s (listed in
// listFile, ffmpeg concat demuxer format) into the final video. Re-encoding
// (not -c copy) so mismatched segment params never corrupt the join.
func concatArgs(listFile, out string) []string {
	return []string{
		"-y",
		"-f", "concat", "-safe", "0", "-i", listFile,
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		out,
	}
}

func sprintfColor(w, h int) string {
	return "color=c=0x0B0B12:s=" + itoa(w) + "x" + itoa(h)
}

// drawtext centers the caption horizontally, near the lower third, wrapped.
func drawtext(captionFile, fontFile string, h int) string {
	y := (h * 72) / 100
	return "drawtext=fontfile=" + fontFile +
		":textfile=" + captionFile +
		":reload=0:fontcolor=white:fontsize=54:line_spacing=12" +
		":box=1:boxcolor=0x000000AA:boxborderw=28" +
		":x=(w-text_w)/2:y=" + itoa(y)
}
