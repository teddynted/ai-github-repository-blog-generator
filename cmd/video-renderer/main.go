// Command video-renderer runs as an ECS Fargate task (one per format). It reads
// the storyboard artifact, synthesizes per-scene narration with Amazon Polly,
// renders a caption + narration MP4 segment per scene with FFmpeg, concatenates
// them at the format's aspect ratio, and uploads the final MP4 to S3.
//
// Inputs are container-environment overrides set by the video state machine:
//
//	FORMAT, SCRIPT_S3_URI, STORYBOARD_S3_URI, VOICEOVER_S3_URI, OUTPUT_S3_URI, ASPECT
//
// v1 drives all formats from the storyboard scenes (single, stable schema) and
// caps short formats to a few scenes; per-format script scene selection and
// richer visuals (images from visual-assets, diagram overlays) are follow-ups.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollytypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	shortSceneCap = 5    // cap scenes for youtube-shorts / tiktok
	maxPollyChars = 2900 // stay within SynthesizeSpeech limits
	defaultVoice  = "Matthew"
	defaultFont   = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	ffmpegBin     = "ffmpeg"
	rsvgBin       = "rsvg-convert"
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

	// Best-effort: rasterize the release's architecture diagram once, to use as
	// the background for architecture/diagram scenes ("" if unavailable).
	diagramPNG := maybeDiagram(ctx, s3c, storyboardURI, work, w, h)

	// Per-scene: narration (Polly) + caption + segment.
	var listBuf bytes.Buffer
	for _, sc := range scenes {
		mp3 := filepath.Join(work, fmt.Sprintf("scene_%d.mp3", sc.Number))
		if err := synthesize(ctx, pollyc, voice, truncate(sc.Narration, maxPollyChars), mp3); err != nil {
			return fmt.Errorf("polly scene %d: %w", sc.Number, err)
		}
		capFile := filepath.Join(work, fmt.Sprintf("scene_%d.txt", sc.Number))
		// Wrap the title to the frame width so the large centred title fits.
		caption := wrapText(sc.Title, 2*w/titleFontsize(w, h))
		if err := os.WriteFile(capFile, []byte(caption), 0o644); err != nil {
			return err
		}
		bg := ""
		if sc.wantsDiagram() {
			bg = diagramPNG // "" falls back to a colour card
		}
		seg := filepath.Join(work, fmt.Sprintf("scene_%d.mp4", sc.Number))
		if err := runFFmpeg(ctx, segmentArgs(capFile, mp3, seg, w, h, fontFile, bg)); err != nil {
			return fmt.Errorf("ffmpeg scene %d: %w", sc.Number, err)
		}
		fmt.Fprintf(&listBuf, "file '%s'\n", seg)
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

	// Upload.
	outBucket, outKey, err := parseS3URI(outputURI)
	if err != nil {
		return err
	}
	if err := putObject(ctx, s3c, outBucket, outKey, final); err != nil {
		return fmt.Errorf("upload final: %w", err)
	}
	log.Printf("uploaded %s", outputURI)
	return nil
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

// maybeDiagram fetches the release's architecture-diagram.svg (derived from the
// storyboard URI) and rasterizes it to a PNG sized to the frame. Best-effort:
// any miss (no diagram, download or rasterize failure) returns "" and the
// renderer falls back to colour cards.
func maybeDiagram(ctx context.Context, s3c *s3.Client, storyboardURI, work string, w, h int) string {
	uri := diagramURIFromStoryboard(storyboardURI)
	if uri == "" {
		return ""
	}
	b, k, err := parseS3URI(uri)
	if err != nil {
		return ""
	}
	svgBytes, err := getObject(ctx, s3c, b, k)
	if err != nil {
		log.Printf("no architecture diagram (%s): %v", uri, err)
		return ""
	}
	// gosec G703 false positive: `work` is the caller's os.MkdirTemp directory
	// (OS-generated, no user input) and the filename is a constant, so there is
	// no path-traversal surface. gosec cannot see this across the function
	// boundary, so the write is annotated below. gosec's directive tag is
	// literally "#nosec" (with the hash) — "//nosec" is not recognized.
	svg := filepath.Join(work, "diagram.svg")
	if err := os.WriteFile(svg, svgBytes, 0o644); err != nil { // #nosec G703 -- work is an os.MkdirTemp dir; filename is constant
		return ""
	}
	png := filepath.Join(work, "diagram.png")
	cmd := exec.CommandContext(ctx, rsvgBin, rsvgArgs(svg, png, w, h)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("rasterize diagram failed: %v", err)
		return ""
	}
	log.Printf("using architecture diagram background: %s", uri)
	return png
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
