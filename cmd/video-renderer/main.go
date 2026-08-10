// Command video-renderer runs as an ECS Fargate task (one per format). It reads
// the storyboard artifact, synthesizes per-scene narration with Amazon Polly,
// renders a caption + narration MP4 segment per scene with FFmpeg, concatenates
// them at the format's aspect ratio, and uploads the final MP4 to S3.
//
// Inputs are container-environment overrides set by the video state machine:
//
//	FORMAT, SCRIPT_S3_URI, STORYBOARD_S3_URI, VOICEOVER_S3_URI, OUTPUT_S3_URI, ASPECT
//
// The render is idempotent: if OUTPUT_S3_URI already exists it is skipped (a
// re-run reuses the existing MP4). Set FORCE_RENDER=true to regenerate anyway.
//
// Each scene's background is — when ENABLE_SCENE_IMAGES is set — an AI-generated
// image from the scene's visual direction (Replicate FLUX/SDXL, or Amazon Nova
// Canvas), falling back to a deep-slate title card on any failure.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollytypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/subtitles"
)

const (
	shortSceneCap = 5 // legacy scene cap (superseded by the duration-based cut)

	// Short-format (youtube-shorts / tiktok) retention tuning: aim for a ~45s
	// vertical cut with one crisp idea per scene, rather than a cropped long-form.
	shortTargetSec         = 45.0 // total runtime target for a Short/TikTok cut
	shortMaxNarrationWords = 15   // per-scene spoken words (≈ one punchy line)
	shortCaptionMaxWords   = 7    // per-scene on-screen caption words (mobile-readable; complete, never truncated)
	shortWordsPerSec       = 2.6  // neural-Polly spoken pace, for duration estimates
	shortScenePadSec       = 0.6  // per-scene silence/transition padding
	maxPollyChars          = 2900 // stay within SynthesizeSpeech limits
	defaultVoice           = "Matthew"
	defaultFont            = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	ffmpegBin              = "ffmpeg"
	ffprobeBin             = "ffprobe"
	rsvgBin                = "rsvg-convert" // rasterizes composed scene diagrams
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("render failed: %v", err)
	}
}

func run(ctx context.Context) error {
	format := getenv("FORMAT", "youtube")
	switch format {
	case "youtube", "youtube-shorts", "tiktok":
	default:
		return fmt.Errorf("unsupported FORMAT %q (want youtube|youtube-shorts|tiktok)", format)
	}
	storyboardURI := os.Getenv("STORYBOARD_S3_URI")
	outputURI := os.Getenv("OUTPUT_S3_URI")
	aspect := getenv("ASPECT", "16:9")
	if storyboardURI == "" || outputURI == "" {
		return fmt.Errorf("STORYBOARD_S3_URI and OUTPUT_S3_URI are required")
	}
	fontFile := getenv("FONT_FILE", defaultFont)
	voice := getenv("VOICE", defaultVoice)

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	s3c := s3.NewFromConfig(cfg)
	pollyc := polly.NewFromConfig(cfg)

	// Idempotency: if this format's MP4 already exists in the video bucket, don't
	// re-render it — skip and exit successfully so a re-run of the pipeline reuses
	// the existing video instead of paying for Polly + FFmpeg (+ image gen) again
	// and overwriting a good render. Set FORCE_RENDER=true to regenerate anyway
	// (e.g. after a renderer change). Checked before any work is done.
	outBucket, outKey, err := parseS3URI(outputURI)
	if err != nil {
		return err
	}
	if !forceRender() {
		exists, err := objectExists(ctx, s3c, outBucket, outKey)
		if err != nil {
			// Don't fail the render on a transient existence-check error — just proceed.
			log.Printf("could not check whether %s exists (%v); rendering", outputURI, err)
		} else if exists {
			log.Printf("video already exists, skipping render: %s (set FORCE_RENDER=true to override)", outputURI)
			return nil
		}
	}

	// Load + parse the storyboard.
	sbBucket, sbKey, err := parseS3URI(storyboardURI)
	if err != nil {
		return err
	}
	sbJSON, err := getObject(ctx, s3c, sbBucket, sbKey)
	if err != nil {
		return fmt.Errorf("download storyboard: %w", err)
	}
	// Short formats (shorts/tiktok) render from their own native vertical script;
	// youtube uses the storyboard it references. Loading the format script is
	// best-effort — a miss falls back to a capped storyboard.
	var scriptJSON []byte
	if scriptURI := os.Getenv("SCRIPT_S3_URI"); scriptURI != "" && (format == "youtube-shorts" || format == "tiktok") {
		if b, k, perr := parseS3URI(scriptURI); perr == nil {
			if body, gerr := getObject(ctx, s3c, b, k); gerr == nil {
				scriptJSON = body
			} else {
				log.Printf("format script unavailable (%s); using storyboard: %v", scriptURI, gerr)
			}
		}
	}
	scenes, err := scenesForFormat(format, scriptJSON, sbJSON)
	if err != nil {
		return fmt.Errorf("select scenes: %w", err)
	}
	if len(scenes) == 0 {
		return fmt.Errorf("no narratable scenes for %s", format)
	}

	// Work in an OS-generated temp dir: no env-derived component ever enters a
	// file path, so there is no path-traversal surface (clears gosec G703). The
	// task is one-shot, so clean up on exit.
	work, err := os.MkdirTemp("", "video-render-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	w, h := dimsFor(aspect)
	log.Printf("rendering %s: %d scenes at %dx%d", format, len(scenes), w, h)

	// Per-scene: narration (Polly) + caption + segment.
	// The opening scene is a SYSTEM-OVERVIEW diagram of the release's architecture
	// (the flow-ordered hero spine collected across all scenes), so the video opens
	// on the whole system rather than a plain title card.
	ovKeys := overviewKeys(scenes)
	// Component adjacency from the release's own content, so a lone-component scene
	// expands into a real flow without hardcoded per-repo relationships.
	adj := coOccurrence(scenes)
	// Build the system-overview loop once. It opens the video AND serves as no-text
	// visual b-roll for any scene that names no service — the video carries NO
	// burned-in captions/overlays; the words live only in the .srt/.ass sidecars.
	overviewMP4 := overviewBackground(ctx, ovKeys, work, w, h)

	var listBuf bytes.Buffer
	// Deterministic captions: accumulate each scene's narration words, timed for the
	// final video, for the .srt/.ass sidecars (script-driven, never transcribed).
	var capWords []subtitles.TimedWord
	offsetMs := 0
	for _, sc := range scenes {
		mp3 := filepath.Join(work, fmt.Sprintf("scene_%d.mp3", sc.Number))
		if err := synthesize(ctx, pollyc, voice, truncate(sc.Narration, maxPollyChars), mp3); err != nil {
			return fmt.Errorf("polly scene %d: %w", sc.Number, err)
		}
		capFile := filepath.Join(work, fmt.Sprintf("scene_%d.txt", sc.Number))
		// Wrap the title to the frame width so the large centred title fits. Use
		// ~1.7 chars-per-fontsize (not 2) so real glyph advances leave a side margin
		// and long words never clip at the frame edge.
		caption := wrapText(sc.Title, (17*w)/(10*titleFontsize(w, h)))
		if err := os.WriteFile(capFile, []byte(caption), 0o644); err != nil {
			return err
		}
		// Background: the opening scene is the system overview; every other scene is a
		// diagram built from the services it names (animated flow, no caption). A scene
		// that names no service falls back to the system-overview loop as visual b-roll
		// — NEVER a text card — so nothing burns text/overlays onto the frame.
		var bg string
		if strings.EqualFold(sc.Type, "title") {
			bg = overviewMP4
		} else if d := diagramBackground(ctx, sc, work, w, h, adj); d != "" {
			bg = d
		} else {
			bg = overviewMP4
		}
		isDiagram := bg != ""
		// A diagram's looped video is bounded to the narration with -t (zoompan and
		// -stream_loop both ignore -shortest); probe the synthesized audio for that
		// length. On any probe miss, durSec is 0 and the clip falls back to -shortest.
		durSec := probeDurationSec(ctx, mp3)
		seg := filepath.Join(work, fmt.Sprintf("scene_%d.mp4", sc.Number))
		if err := runFFmpeg(ctx, segmentArgs(capFile, mp3, seg, w, h, fontFile, bg, "", durSec, isDiagram)); err != nil {
			return fmt.Errorf("ffmpeg scene %d: %w", sc.Number, err)
		}
		fmt.Fprintf(&listBuf, "file '%s'\n", seg)
		// Timed caption words for the sidecar subtitles (offset by the scene's start).
		durMs := int(durSec * 1000)
		capWords = append(capWords, sceneCaptionWords(ctx, pollyc, voice, sc.Narration, offsetMs, durMs)...)
		offsetMs += durMs
	}

	// Concatenate into the final MP4.
	listFile := filepath.Join(work, "concat.txt")
	if err := os.WriteFile(listFile, listBuf.Bytes(), 0o644); err != nil {
		return err
	}
	final := filepath.Join(work, "final.mp4")
	if err := runFFmpeg(ctx, concatArgs(listFile, final)); err != nil {
		return fmt.Errorf("ffmpeg concat: %w", err)
	}

	// Upload (outBucket/outKey parsed above for the idempotency check).
	if err := putObject(ctx, s3c, outBucket, outKey, final); err != nil {
		return fmt.Errorf("upload final: %w", err)
	}
	log.Printf("uploaded %s", outputURI)

	// Deterministic caption sidecars (.srt/.ass) next to the MP4 — for platform CC /
	// upload / accessibility. Not burned into the frame (the video carries no text —
	// diagram/overview b-roll only; the words live solely in these sidecars).
	writeSubtitles(ctx, s3c, capWords, work, outBucket, outKey, w, h)
	return nil
}

// probeDurationSec returns the duration of an audio file in seconds via ffprobe,
// or 0 on any error (the caller then bounds the clip with -shortest instead).
func probeDurationSec(ctx context.Context, path string) float64 {
	out, err := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return d
}

func synthesize(ctx context.Context, p *polly.Client, voice, text, outMP3 string) error {
	out, err := p.SynthesizeSpeech(ctx, &polly.SynthesizeSpeechInput{
		Engine:       pollytypes.EngineNeural,
		OutputFormat: pollytypes.OutputFormatMp3,
		VoiceId:      pollytypes.VoiceId(voice),
		Text:         aws.String(text),
	})
	if err != nil {
		return err
	}
	defer out.AudioStream.Close()
	f, err := os.Create(outMP3)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, out.AudioStream)
	return err
}

func runFFmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// forceRender reports whether FORCE_RENDER=true, which bypasses the
// already-exists skip so a format is re-rendered even when its MP4 is present.
func forceRender() bool { return strings.EqualFold(os.Getenv("FORCE_RENDER"), "true") }

// objectExists reports whether an S3 object is present. A NotFound (404) is a
// clean "no"; any other error is returned so the caller can decide.
func objectExists(ctx context.Context, c *s3.Client, bucket, key string) (bool, error) {
	_, err := c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var nf *s3types.NotFound
	if errors.As(err, &nf) {
		return false, nil
	}
	// HeadObject sometimes surfaces the 404 as a generic response error rather than
	// the modelled NotFound type, so fall back to matching the status/code text.
	msg := err.Error()
	if strings.Contains(msg, "NotFound") || strings.Contains(msg, "status code: 404") {
		return false, nil
	}
	return false, err
}

func getObject(ctx context.Context, c *s3.Client, bucket, key string) ([]byte, error) {
	out, err := c.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func putObject(ctx context.Context, c *s3.Client, bucket, key, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String("video/mp4"),
	})
	return err
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
