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

// segmentArgs builds the ffmpeg args that render one scene into an MP4: a
// background sized to the format (the AI-generated scene image when bgImage is
// set, otherwise a deep-slate title slide), the scene caption burned in via
// drawtext (read from a file to avoid escaping), and the narration audio.
// -shortest makes the clip exactly as long as the narration.
func segmentArgs(captionFile, narrationMP3, out string, w, h int, fontFile, bgImage string) []string {
	var inputs []string
	var vf string
	if bgImage != "" {
		// Loop the scene image; cover the frame (scale up + centre-crop) then put
		// the caption in a lower band so it stays readable over the image.
		inputs = []string{"-loop", "1", "-i", bgImage}
		vf = "scale=" + itoa(w) + ":" + itoa(h) + ":force_original_aspect_ratio=increase,crop=" + itoa(w) + ":" + itoa(h) + "," + captionBand(captionFile, fontFile, w, h)
	} else {
		// No image: a proper title slide — deep-slate background with the scene
		// title large and centred, so it reads as a designed slide, not a black card.
		inputs = []string{"-f", "lavfi", "-i", sprintfColor(w, h)}
		vf = titleCard(captionFile, fontFile, w, h)
	}
	args := []string{"-y"}
	args = append(args, inputs...)
	return append(args,
		"-i", narrationMP3,
		"-vf", vf,
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k",
		"-shortest",
		out,
	)
}

// titleFontsize scales the title to the frame (min dimension), so 16:9 and 9:16
// both get a large, readable title.
func titleFontsize(w, h int) int {
	m := w
	if h < w {
		m = h
	}
	return m / 11
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
	// Deep slate — reads as an intentional slide, not a black screen.
	return "color=c=0x0F172A:s=" + itoa(w) + "x" + itoa(h)
}

// titleCard renders the scene title large and centred on both axes with a soft
// box — so a caption-only scene looks like a designed title slide.
func titleCard(captionFile, fontFile string, w, h int) string {
	fs := titleFontsize(w, h)
	return "drawtext=fontfile=" + fontFile +
		":textfile=" + captionFile +
		":reload=0:fontcolor=white:fontsize=" + itoa(fs) +
		":line_spacing=" + itoa(fs/4) +
		":box=1:boxcolor=0x000000AA:boxborderw=" + itoa(fs/2) +
		":x=(w-text_w)/2:y=(h-text_h)/2"
}

// captionBand renders the caption smaller in the lower third, for scenes that
// have an image background (kept legible with a stronger box).
func captionBand(captionFile, fontFile string, w, h int) string {
	fs := titleFontsize(w, h) * 2 / 3
	y := (h * 74) / 100
	return "drawtext=fontfile=" + fontFile +
		":textfile=" + captionFile +
		":reload=0:fontcolor=white:fontsize=" + itoa(fs) +
		":line_spacing=" + itoa(fs/4) +
		":box=1:boxcolor=0x000000CC:boxborderw=" + itoa(fs/2) +
		":x=(w-text_w)/2:y=" + itoa(y)
}
