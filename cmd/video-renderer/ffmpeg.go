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
// background sized to the format (an animated architecture-diagram loop, or a
// deep-slate title/heading card), plus the narration audio.
//
// Both streams are forced to EXACTLY the narration length durSec: the audio is
// padded with apad and the output bounded with -t (so it is never short), and the
// video runs at a constant -r 30 bounded by the same -t. This exact per-segment
// match is essential — the concat demuxer that stitches the scenes together
// drifts badly when a segment's video and audio lengths differ (that desynced the
// final video by ~17s). -shortest is only a last-resort fallback when the
// narration length is unknown (durSec<=0); it is unreliable with the infinite
// video sources used here (a -stream_loop diagram or an infinite lavfi color).
func segmentArgs(captionFile, narrationMP3, out string, w, h int, fontFile, bgImage, move string, durSec float64, animated bool) []string {
	var inputs []string
	var vf string
	switch {
	case animated && bgImage != "":
		// Animated architecture diagram (or overview b-roll), looped, NO caption
		// overlaid — the video carries no burned-in text; captions live in the sidecars.
		inputs = []string{"-stream_loop", "-1", "-i", bgImage}
		vf = ""
	case bgImage != "":
		// Legacy still-image path (retained for parity; AI scene photos are no longer
		// used). Loop the image with an optional Ken Burns move and a lower-third band.
		inputs = []string{"-loop", "1", "-i", bgImage}
		motion := ""
		if move != "" && durSec > 0 {
			motion = motionFilter(move, w, h, int(durSec*30+0.5))
		}
		bg := motion
		if bg == "" {
			bg = "scale=" + itoa(w) + ":" + itoa(h) + ":force_original_aspect_ratio=increase,crop=" + itoa(w) + ":" + itoa(h)
		}
		vf = bg + "," + captionBand(captionFile, fontFile, w, h)
	default:
		// Clean full-screen CARD: deep-slate background with the scene's title/heading
		// large and centred. Both the opening title card and any scene with no diagram.
		inputs = []string{"-f", "lavfi", "-i", sprintfColor(w, h)}
		vf = titleCard(captionFile, fontFile, w, h)
	}
	args := []string{"-y"}
	args = append(args, inputs...)
	args = append(args, "-i", narrationMP3, "-map", "0:v", "-map", "1:a")
	if vf != "" {
		args = append(args, "-vf", vf)
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "160k")
	if durSec > 0 {
		// Pad audio + constant fps + a shared output -t → video and audio are BOTH
		// exactly durSec, so concatenated segments stay in sync.
		args = append(args, "-af", "apad", "-r", "30", "-t", strconv.FormatFloat(durSec, 'f', 3, 64))
	} else {
		args = append(args, "-shortest") // fallback only when the narration length is unknown
	}
	return append(args, out)
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

// motionFilter builds a Ken Burns zoompan filter chain that gives a still scene
// image a slow, cinematic camera move, output at the frame size (WxH). The image
// is over-scaled first for pan/zoom headroom. move selects the motion; "" means
// motion is disabled and the caller uses a static cover crop instead.
//
// The zoom accumulates per output frame (d=1) against a looped image, so the move
// runs for the scene's full narration-driven length (the segment is -shortest).
func motionFilter(move string, w, h, frames int) string {
	if move == "" || frames <= 0 {
		return ""
	}
	// Over-scale to 1.5x the frame so zooming/cropping never runs out of pixels.
	cover := "scale=" + itoa(w*3/2) + ":" + itoa(h*3/2) + ":force_original_aspect_ratio=increase,crop=" + itoa(w*3/2) + ":" + itoa(h*3/2)
	center := ":x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)'"
	zp := func(z string) string {
		return cover + ",zoompan=z='" + z + "':d=" + itoa(frames) + center + ":s=" + itoa(w) + "x" + itoa(h) + ":fps=30"
	}
	switch move {
	case "dolly_out":
		return zp("if(eq(on,1),1.12,max(zoom-0.0009,1.0))")
	case "push_in":
		return zp("min(zoom+0.0014,1.16)")
	case "drift_in":
		return zp("min(zoom+0.0006,1.08)")
	default: // dolly_in
		return zp("min(zoom+0.0010,1.12)")
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
