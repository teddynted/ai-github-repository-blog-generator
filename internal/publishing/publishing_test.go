package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- fixtures ---

func approvedBlog() Content {
	return Content{
		ID:             "blog-v0.2.0",
		Type:           TypeBlog,
		Title:          "Inside widget v0.2.0",
		Body:           "# Inside widget v0.2.0\n\nA deep dive into the release context builder.",
		Tags:           []string{"go", "aws", "serverless", "devops", "architecture", "cloud"},
		CanonicalURL:   "https://widget.dev/blog/inside-widget-v0-2-0",
		CoverImage:     "https://widget.dev/cover.png",
		Description:    "A deep dive into widget v0.2.0.",
		ReleaseVersion: "v0.2.0",
		Author:         "author@example.com",
		Approved:       true,
		ApprovalRef:    "governance:blog-v0.2.0",
		Metadata:       map[string]string{"thumbnail": "t.png"},
	}
}

func fixedClock() Clock {
	t := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	return func() time.Time { t = t.Add(time.Millisecond); return t }
}

func instantSleep() Sleeper { return func(context.Context, time.Duration) {} }

func newEngine(t *testing.T, cfg Config, pubs ...Publisher) *Engine {
	t.Helper()
	e := NewEngine(cfg, NewMemoryRepository(), pubs, fixedClock())
	e.Sleep = instantSleep()
	return e
}

// --- approval gate ---

func TestNeverPublishesUnapproved(t *testing.T) {
	e := newEngine(t, DefaultConfig(), NewDryRunPublisher(PlatformDevTo))
	c := approvedBlog()
	c.Approved = false
	if _, err := e.Distribute(context.Background(), c, DistributeRequest{Targets: []Platform{PlatformDevTo}}); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("expected ErrNotApproved, got %v", err)
	}
}

// --- multi-platform distribution ---

func TestDistributeMultiPlatformInOrder(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	med := NewDryRunPublisher(PlatformMedium)
	gh := NewDryRunPublisher(PlatformGitHub)
	e := newEngine(t, DefaultConfig(), med, dev, gh) // registered out of order

	pubs, err := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{})
	if err != nil {
		t.Fatalf("distribute: %v", err)
	}
	// All three support blogs; every one published.
	if len(pubs) != 3 {
		t.Fatalf("expected 3 publications, got %d", len(pubs))
	}
	for _, p := range pubs {
		if p.Status != StatusPublished {
			t.Errorf("%s status = %q, want Published (%v)", p.Platform, p.Status, p.Errors)
		}
		if p.URL == "" || p.PlatformID == "" {
			t.Errorf("%s missing url/id", p.Platform)
		}
	}
	// Config priority order: GitHub, Dev.to, ..., Medium.
	if pubs[0].Platform != PlatformGitHub || pubs[len(pubs)-1].Platform != PlatformMedium {
		t.Errorf("order not honoured: %s ... %s", pubs[0].Platform, pubs[len(pubs)-1].Platform)
	}
}

func TestUnsupportedTypeSkipped(t *testing.T) {
	// A blog targeted at YouTube (unsupported) fails validation, not a panic.
	yt := NewDryRunPublisher(PlatformYouTube)
	yt.SupportsFn = func(t ContentType) bool { return t == TypeYouTubeVideo }
	e := newEngine(t, DefaultConfig(), yt)
	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformYouTube}})
	if len(pubs) != 1 || pubs[0].Status != StatusFailed {
		t.Fatalf("expected a failed publication for unsupported type, got %+v", pubs)
	}
	if !strings.Contains(strings.Join(pubs[0].Errors, " "), "does not support") {
		t.Errorf("expected unsupported error, got %v", pubs[0].Errors)
	}
}

// --- retry ---

func TestRetryRecoverableThenSucceeds(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	dev.FailTimes = 2
	dev.FailWith = errRateLimit("slow down") // recoverable
	cfg := DefaultConfig()
	cfg.Retry = RetryPolicy{MaxRetries: 3, BaseDelay: time.Millisecond, Multiplier: 2}
	e := newEngine(t, cfg, dev)

	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	p := pubs[0]
	if p.Status != StatusPublished {
		t.Fatalf("should publish after retries, got %q (%v)", p.Status, p.Errors)
	}
	if p.RetryCount != 2 {
		t.Errorf("retryCount = %d, want 2", p.RetryCount)
	}
	if len(p.Attempts) != 3 {
		t.Errorf("attempts = %d, want 3", len(p.Attempts))
	}
}

func TestPermanentErrorNotRetried(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	dev.FailTimes = 5
	dev.FailWith = errAuth("bad token") // permanent
	cfg := DefaultConfig()
	cfg.Retry = RetryPolicy{MaxRetries: 5, BaseDelay: time.Millisecond}
	e := newEngine(t, cfg, dev)

	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	p := pubs[0]
	if p.Status != StatusFailed {
		t.Fatalf("permanent error should fail, got %q", p.Status)
	}
	if len(p.Attempts) != 1 {
		t.Errorf("permanent error should not retry: attempts = %d", len(p.Attempts))
	}
}

func TestRetryEngineBackoff(t *testing.T) {
	e := RetryEngine{Policy: RetryPolicy{BaseDelay: time.Second, Multiplier: 2, MaxDelay: 10 * time.Second}}
	got := []time.Duration{e.backoff(0), e.backoff(1), e.backoff(2), e.backoff(10)}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 10 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("backoff(%d) = %v, want %v", i, got[i], want[i])
		}
	}
}

// --- scheduling ---

func TestScheduledPublicationParkedThenRunDue(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	e := NewEngine(DefaultConfig(), NewMemoryRepository(), []Publisher{dev}, clock)
	e.Sleep = instantSleep()

	future := now.Add(time.Hour)
	pubs, _ := e.Distribute(context.Background(), approvedBlog(),
		DistributeRequest{Targets: []Platform{PlatformDevTo}, Schedule: Schedule{Mode: ScheduleAt, At: future}})
	if pubs[0].Status != StatusScheduled || pubs[0].ScheduledFor == nil {
		t.Fatalf("expected Scheduled, got %q", pubs[0].Status)
	}

	// Not due yet.
	if n, _ := e.RunDue(context.Background(), contentLookup(approvedBlog())); n != 0 {
		t.Errorf("nothing should be due yet, ran %d", n)
	}
	// Advance past the scheduled time.
	now = future.Add(time.Minute)
	if n, _ := e.RunDue(context.Background(), contentLookup(approvedBlog())); n != 1 {
		t.Fatalf("expected 1 due publication to run")
	}
	p, _ := e.Get("blog-v0.2.0:Dev.to")
	if p.Status != StatusPublished {
		t.Errorf("due publication should be Published, got %q", p.Status)
	}
}

func TestSchedulerBusinessHours(t *testing.T) {
	s := Scheduler{}
	// A delay landing at 03:00 UTC should shift to the 09:00 window start.
	base := time.Date(2026, 7, 20, 2, 0, 0, 0, time.UTC)
	sc := Schedule{Mode: ScheduleDelay, Delay: time.Hour, TimeZone: "UTC", Window: &BusinessHours{StartHour: 9, EndHour: 17}}
	got := s.NextRun(sc, base)
	if got.Hour() != 9 {
		t.Errorf("expected shift into business hours (09:00), got %v", got)
	}
}

// --- metadata limits ---

func TestMetadataPerPlatformLimits(t *testing.T) {
	me := MetadataEngine{Config: DefaultConfig()}
	c := approvedBlog()
	c.Title = strings.Repeat("x", 200)

	dev := me.For(PlatformDevTo, c)
	if len([]rune(dev.Title)) > devtoTitleMax || len(dev.Tags) > devtoTagMax {
		t.Errorf("dev.to limits not applied: title %d, tags %d", len([]rune(dev.Title)), len(dev.Tags))
	}
	med := me.For(PlatformMedium, c)
	if len(med.Tags) > mediumTagMax || med.Extra["publishStatus"] == "" {
		t.Errorf("medium metadata wrong: %+v", med)
	}
	yt := me.For(PlatformYouTube, c)
	if len([]rune(yt.Title)) > ytTitleMax || yt.Extra["category"] == "" {
		t.Errorf("youtube metadata wrong: %+v", yt)
	}
	gh := me.For(PlatformGitHub, c)
	if gh.Extra["path"] == "" || gh.Extra["commitMessage"] == "" {
		t.Errorf("github metadata wrong: %+v", gh)
	}
}

// --- validation ---

func TestValidationRejectsUnapprovedAndInvalid(t *testing.T) {
	v := ValidationEngine{}
	dev := NewDryRunPublisher(PlatformDevTo)
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)

	c.Approved = false
	if err := v.Validate(c, m, dev); !errors.Is(err, ErrNotApproved) {
		t.Errorf("expected ErrNotApproved, got %v", err)
	}
	c.Approved = true
	m.Body = ""
	if err := v.Validate(c, m, dev); err == nil {
		t.Error("expected error for empty article body")
	}
}

// --- adapters (mock HTTP) ---

type mockDoer struct {
	status  int
	body    string
	gotReq  *http.Request
	gotBody string
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.gotReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		m.gotBody = string(b)
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func TestDevToAdapterBuildsRequest(t *testing.T) {
	doer := &mockDoer{status: 201, body: `{"id":123,"url":"https://dev.to/acme/inside-widget"}`}
	pub := NewDevToPublisher(doer, mapCredentials(map[string]string{envDevToKey: "secret"}))
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)

	res, err := pub.Publish(context.Background(), c, m)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if res.PlatformID != "123" || res.URL == "" {
		t.Errorf("result = %+v", res)
	}
	// The api-key header is sent; body carries markdown + tags.
	if doer.gotReq.Header.Get("api-key") != "secret" {
		t.Error("api-key header not set")
	}
	if !strings.Contains(doer.gotBody, "body_markdown") || !strings.Contains(doer.gotBody, "canonical_url") {
		t.Errorf("request body missing fields: %s", doer.gotBody)
	}
}

func TestAdapterClassifiesHTTPErrors(t *testing.T) {
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)
	cases := map[int]struct {
		code        string
		recoverable bool
	}{
		401: {"auth", false},
		429: {"rate-limit", true},
		409: {"duplicate", false},
		500: {"platform-down", true},
	}
	for status, want := range cases {
		doer := &mockDoer{status: status, body: `{"error":"x"}`}
		pub := NewDevToPublisher(doer, mapCredentials(map[string]string{envDevToKey: "k"}))
		_, err := pub.Publish(context.Background(), c, m)
		if errCode(err) != want.code {
			t.Errorf("status %d → code %q, want %q", status, errCode(err), want.code)
		}
		if isRecoverable(err) != want.recoverable {
			t.Errorf("status %d recoverable = %v, want %v", status, isRecoverable(err), want.recoverable)
		}
	}
}

func TestHashnodeGraphQL(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"data":{"publishPost":{"post":{"id":"p1","url":"https://blog.hashnode.dev/x"}}}}`}
	pub := NewHashnodePublisher(doer, mapCredentials(map[string]string{envHashnodeToken: "t"}))
	c := approvedBlog()
	c.Metadata = map[string]string{"hashnodePublicationId": "pub123"}
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformHashnode, c)
	if err := pub.Validate(c, m); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := pub.Publish(context.Background(), c, m)
	if err != nil || res.PlatformID != "p1" {
		t.Fatalf("hashnode publish: %v, %+v", err, res)
	}
	if !strings.Contains(doer.gotBody, "publishPost") || !strings.Contains(doer.gotBody, "pub123") {
		t.Errorf("graphql body missing mutation/publicationId: %s", doer.gotBody)
	}
}

func TestGitHubContentsUpsert(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"content":{"html_url":"https://github.com/acme/widget/blob/main/docs/x.md","sha":"abc","path":"docs/x.md"}}`}
	pub := NewGitHubPublisher(doer, mapCredentials(map[string]string{envGitHubToken: "t"}), "acme", "widget")
	c := approvedBlog()
	c.Type = TypeBlog
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformGitHub, c)
	res, err := pub.Publish(context.Background(), c, m)
	if err != nil || res.URL == "" {
		t.Fatalf("github publish: %v, %+v", err, res)
	}
	if !strings.Contains(doer.gotBody, "content") { // base64 body under "content"
		t.Errorf("github body missing content: %s", doer.gotBody)
	}
}

func TestYouTubeShortsTitleAndVisibility(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"id":"vid123"}`}
	pub := NewYouTubePublisher(doer, mapCredentials(map[string]string{envYouTubeToken: "t"}))
	c := approvedBlog()
	c.Type = TypeYouTubeShorts
	c.Assets = []string{"video.mp4"}
	c.Metadata = map[string]string{"visibility": "unlisted", "videoId": "vid123"}
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformYouTube, c)
	res, err := pub.Publish(context.Background(), c, m)
	if err != nil || res.PlatformID != "vid123" {
		t.Fatalf("youtube publish: %v, %+v", err, res)
	}
	if !strings.Contains(doer.gotBody, "#Shorts") || !strings.Contains(doer.gotBody, "unlisted") {
		t.Errorf("youtube body missing shorts tag/visibility: %s", doer.gotBody)
	}
}

func TestMediumAdapter(t *testing.T) {
	doer := &mockDoer{status: 201, body: `{"data":{"id":"m1","url":"https://medium.com/@a/x"}}`}
	pub := NewMediumPublisher(doer, mapCredentials(map[string]string{envMediumToken: "t", envMediumUserID: "u1"}))
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformMedium, c)
	if err := pub.Validate(c, m); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := pub.Publish(context.Background(), c, m)
	if err != nil || res.PlatformID != "m1" {
		t.Fatalf("medium publish: %v, %+v", err, res)
	}
	if !strings.Contains(doer.gotReq.URL.Path, "/users/u1/posts") {
		t.Errorf("medium URL wrong: %s", doer.gotReq.URL.Path)
	}
	if doer.gotReq.Header.Get("Authorization") != "Bearer t" {
		t.Error("medium bearer token not set")
	}
	// Update/Delete are unsupported by the API.
	if _, err := pub.Update(context.Background(), "m1", c, m); errCode(err) != "unsupported" {
		t.Errorf("expected unsupported update, got %v", err)
	}
	if err := pub.Delete(context.Background(), "m1"); errCode(err) != "unsupported" {
		t.Errorf("expected unsupported delete, got %v", err)
	}
}

func TestDevToUpdateDeleteStatus(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"id":9,"url":"https://dev.to/x","published":true}`}
	pub := NewDevToPublisher(doer, mapCredentials(map[string]string{envDevToKey: "k"}))
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)
	if _, err := pub.Update(context.Background(), "9", c, m); err != nil {
		t.Errorf("update: %v", err)
	}
	if err := pub.Delete(context.Background(), "9"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if s, err := pub.GetStatus(context.Background(), "9"); err != nil || s != "published" {
		t.Errorf("status = %q, %v", s, err)
	}
	// Missing credential fails validation.
	nope := NewDevToPublisher(doer, mapCredentials(nil))
	if err := nope.Validate(c, m); errCode(err) != "auth" {
		t.Errorf("expected auth error without key, got %v", err)
	}
}

func TestYouTubeStatusAndDelete(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"items":[{"status":{"uploadStatus":"processed","privacyStatus":"public"}}]}`}
	pub := NewYouTubePublisher(doer, mapCredentials(map[string]string{envYouTubeToken: "t"}))
	if s, err := pub.GetStatus(context.Background(), "vid"); err != nil || !strings.Contains(s, "processed") {
		t.Errorf("status = %q, %v", s, err)
	}
	del := &mockDoer{status: 204, body: ""}
	pub2 := NewYouTubePublisher(del, mapCredentials(map[string]string{envYouTubeToken: "t"}))
	if err := pub2.Delete(context.Background(), "vid"); err != nil {
		t.Errorf("delete: %v", err)
	}
}

func TestDisabledPublisherFails(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = []Platform{PlatformMedium} // Dev.to disabled
	dev := NewDryRunPublisher(PlatformDevTo)
	e := newEngine(t, cfg, dev)
	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	if pubs[0].Status != StatusFailed {
		t.Errorf("disabled publisher should fail, got %q", pubs[0].Status)
	}
}

func TestContinueOnErrorFalseStops(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContinueOnError = false
	cfg.Retry = RetryPolicy{MaxRetries: 0, BaseDelay: time.Millisecond}
	gh := NewDryRunPublisher(PlatformGitHub)
	gh.FailTimes = 1
	gh.FailWith = errAuth("nope")
	dev := NewDryRunPublisher(PlatformDevTo)
	e := newEngine(t, cfg, gh, dev)
	// GitHub is first by priority and fails permanently → stop before Dev.to.
	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{})
	if len(pubs) != 1 || pubs[0].Platform != PlatformGitHub || pubs[0].Status != StatusFailed {
		t.Errorf("expected to stop after GitHub failure, got %+v", pubs)
	}
}

func TestLogNotifier(t *testing.T) {
	if err := (LogNotifier{}).Notify(context.Background(), Event{Kind: EventPublished, Platform: PlatformDevTo}); err != nil {
		t.Errorf("log notifier: %v", err)
	}
}

// --- tracking / status ---

func TestCancelAndReport(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	e := NewEngine(DefaultConfig(), NewMemoryRepository(), []Publisher{dev}, func() time.Time { return now })
	e.Sleep = instantSleep()

	// Schedule far in the future, then cancel.
	pubs, _ := e.Distribute(context.Background(), approvedBlog(),
		DistributeRequest{Targets: []Platform{PlatformDevTo}, Schedule: Schedule{Mode: ScheduleAt, At: now.Add(time.Hour)}})
	p, err := e.Cancel(pubs[0].ID)
	if err != nil || p.Status != StatusCancelled {
		t.Fatalf("cancel: %v, status %q", err, p.Status)
	}
	// Report renders.
	all, _ := e.List()
	md := Report(all)
	for _, want := range []string{"# Publication Report", "| Platform | Status", "Audit trail"} {
		if !strings.Contains(md, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestStatusMachineRejectsIllegal(t *testing.T) {
	tr := Tracker{Repo: NewMemoryRepository(), Now: fixedClock()}
	p := &Publication{ID: "x", Status: StatusPublished}
	_ = tr.Repo.Save(p)
	// Published → Publishing is illegal.
	if err := tr.transition(p, StatusPublishing, "x", ""); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

// --- notifications ---

type spyNotifier struct{ kinds []string }

func (s *spyNotifier) Notify(_ context.Context, ev Event) error {
	s.kinds = append(s.kinds, ev.Kind)
	return nil
}

func TestNotificationsEmitted(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	spy := &spyNotifier{}
	e := newEngine(t, DefaultConfig(), dev)
	e.Notifier = spy
	_, _ = e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	if !contains(spy.kinds, EventPublished) {
		t.Errorf("expected a published event, got %v", spy.kinds)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	e := newEngine(t, DefaultConfig(), NewDryRunPublisher(PlatformDevTo))
	pubs, _ := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	if _, err := json.Marshal(pubs); err != nil {
		t.Errorf("marshal: %v", err)
	}
}

func TestSchedulerModes(t *testing.T) {
	s := Scheduler{}
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if got := s.NextRun(Schedule{Mode: ScheduleImmediate}, base); !got.Equal(base) {
		t.Errorf("immediate should run now, got %v", got)
	}
	if got := s.NextRun(Schedule{Mode: ScheduleDelay, Delay: 2 * time.Hour}, base); !got.Equal(base.Add(2 * time.Hour)) {
		t.Errorf("delay wrong: %v", got)
	}
	past := s.NextRun(Schedule{Mode: ScheduleAt, At: base.Add(-time.Hour)}, base)
	if past.Before(base) {
		t.Errorf("past 'at' should clamp to now, got %v", past)
	}
	rec := s.NextRun(Schedule{Mode: ScheduleRecurring}, base)
	if !rec.After(base) {
		t.Errorf("recurring should be in the future, got %v", rec)
	}
}

func TestRunDueMissingContentFails(t *testing.T) {
	dev := NewDryRunPublisher(PlatformDevTo)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	e := NewEngine(DefaultConfig(), NewMemoryRepository(), []Publisher{dev}, func() time.Time { return now })
	e.Sleep = instantSleep()
	_, _ = e.Distribute(context.Background(), approvedBlog(),
		DistributeRequest{Targets: []Platform{PlatformDevTo}, Schedule: Schedule{Mode: ScheduleAt, At: now.Add(time.Hour)}})
	now = now.Add(2 * time.Hour)
	// Content lookup returns nothing → the due publication fails.
	_, _ = e.RunDue(context.Background(), func(string) (Content, bool) { return Content{}, false })
	p, _ := e.Get("blog-v0.2.0:Dev.to")
	if p.Status != StatusFailed {
		t.Errorf("missing content should fail the scheduled publication, got %q", p.Status)
	}
}

func TestRegisterAndNoPublisher(t *testing.T) {
	e := newEngine(t, DefaultConfig())
	// No publisher registered → no targets.
	if _, err := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{}); !errors.Is(err, ErrNoPublisher) {
		t.Errorf("expected ErrNoPublisher, got %v", err)
	}
	// Register one and it works.
	e.Register(NewDryRunPublisher(PlatformDevTo))
	pubs, err := e.Distribute(context.Background(), approvedBlog(), DistributeRequest{})
	if err != nil || len(pubs) != 1 {
		t.Errorf("after register: %v, %d pubs", err, len(pubs))
	}
}

func TestAdapterGetStatusPaths(t *testing.T) {
	hn := NewHashnodePublisher(&mockDoer{status: 200, body: "{}"}, mapCredentials(nil))
	if s, _ := hn.GetStatus(context.Background(), "p"); s != "published" {
		t.Errorf("hashnode status = %q", s)
	}
	md := NewMediumPublisher(&mockDoer{status: 200, body: "{}"}, mapCredentials(nil))
	if s, _ := md.GetStatus(context.Background(), "m"); s != "published" {
		t.Errorf("medium status = %q", s)
	}
	gh := NewGitHubPublisher(&mockDoer{status: 200, body: `{"sha":"abc"}`}, mapCredentials(map[string]string{envGitHubToken: "t"}), "a", "b")
	if s, err := gh.GetStatus(context.Background(), "docs/x.md"); err != nil || s != "published" {
		t.Errorf("github status = %q, %v", s, err)
	}
}

func TestMoreHTTPStatusCodes(t *testing.T) {
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)
	for status, code := range map[int]string{403: "auth", 402: "quota", 400: "invalid"} {
		pub := NewDevToPublisher(&mockDoer{status: status, body: `{"error":"x"}`}, mapCredentials(map[string]string{envDevToKey: "k"}))
		if _, err := pub.Publish(context.Background(), c, m); errCode(err) != code {
			t.Errorf("status %d → %q, want %q", status, errCode(err), code)
		}
	}
}

func TestHashnodeAndGitHubUpdateDelete(t *testing.T) {
	hn := NewHashnodePublisher(&mockDoer{status: 200, body: "{}"}, mapCredentials(nil))
	if _, err := hn.Update(context.Background(), "p", approvedBlog(), PlatformMetadata{}); errCode(err) != "unsupported" {
		t.Errorf("hashnode update should be unsupported, got %v", err)
	}
	if err := hn.Delete(context.Background(), "p"); errCode(err) != "unsupported" {
		t.Errorf("hashnode delete should be unsupported, got %v", err)
	}
	gh := NewGitHubPublisher(&mockDoer{status: 200, body: `{"content":{"html_url":"u","sha":"s"}}`}, mapCredentials(map[string]string{envGitHubToken: "t"}), "a", "b")
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformGitHub, c)
	if _, err := gh.Update(context.Background(), "docs/x.md", c, m); err != nil {
		t.Errorf("github update: %v", err)
	}
	del := NewGitHubPublisher(&mockDoer{status: 200, body: `{"sha":"s"}`}, mapCredentials(map[string]string{envGitHubToken: "t"}), "a", "b")
	if err := del.Delete(context.Background(), "docs/x.md"); err != nil {
		t.Errorf("github delete: %v", err)
	}
}

func TestMetadataHashnodeAndTagBudget(t *testing.T) {
	c := approvedBlog()
	c.Metadata = map[string]string{"hashnodePublicationId": "pub9"}
	hn := MetadataEngine{Config: DefaultConfig()}.For(PlatformHashnode, c)
	if hn.Extra["publicationId"] != "pub9" || hn.Extra["slug"] == "" {
		t.Errorf("hashnode metadata wrong: %+v", hn.Extra)
	}
	got := fitTagBudget([]string{"aaaa", "bbbb", "cccc"}, 10)
	if len(got) != 2 { // 4+1 + 4+1 = 10, third would exceed
		t.Errorf("fitTagBudget = %v, want 2 tags", got)
	}
}

func TestYouTubeInsertWithoutVideoID(t *testing.T) {
	// No videoId → POST (insert). Uses a video file asset.
	doer := &mockDoer{status: 200, body: `{"id":"newvid"}`}
	pub := NewYouTubePublisher(doer, mapCredentials(map[string]string{envYouTubeToken: "t"}))
	c := approvedBlog()
	c.Type = TypeYouTubeVideo
	c.Assets = []string{"video.mp4"}
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformYouTube, c)
	res, err := pub.Publish(context.Background(), c, m)
	if err != nil || res.PlatformID != "newvid" {
		t.Fatalf("youtube insert: %v, %+v", err, res)
	}
	if doer.gotReq.Method != "POST" {
		t.Errorf("insert should POST, got %s", doer.gotReq.Method)
	}
}

type errDoer struct{}

func (errDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp: connection refused")
}

func TestTransportErrorIsRecoverableNetwork(t *testing.T) {
	pub := NewDevToPublisher(errDoer{}, mapCredentials(map[string]string{envDevToKey: "k"}))
	c := approvedBlog()
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformDevTo, c)
	_, err := pub.Publish(context.Background(), c, m)
	if errCode(err) != "network" || !isRecoverable(err) {
		t.Errorf("transport error should be recoverable network, got %q (recoverable=%v)", errCode(err), isRecoverable(err))
	}
}

func TestYouTubeDescriptionWithChapters(t *testing.T) {
	c := approvedBlog()
	c.Type = TypeYouTubeVideo
	c.Metadata = map[string]string{"chapters": "00:00 Intro\n01:00 Architecture"}
	m := MetadataEngine{Config: DefaultConfig()}.For(PlatformYouTube, c)
	if !strings.Contains(m.Description, "Chapters:") || !strings.Contains(m.Description, "Intro") {
		t.Errorf("youtube description should include chapters: %q", m.Description)
	}
}

func TestImmediateScheduleModePublishes(t *testing.T) {
	e := newEngine(t, DefaultConfig(), NewDryRunPublisher(PlatformDevTo))
	pubs, _ := e.Distribute(context.Background(), approvedBlog(),
		DistributeRequest{Targets: []Platform{PlatformDevTo}, Schedule: Schedule{Mode: ScheduleImmediate}})
	if pubs[0].Status != StatusPublished {
		t.Errorf("immediate schedule should publish now, got %q", pubs[0].Status)
	}
}

func TestGetCancelNotFound(t *testing.T) {
	e := newEngine(t, DefaultConfig(), NewDryRunPublisher(PlatformDevTo))
	if _, err := e.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: expected ErrNotFound, got %v", err)
	}
	if _, err := e.Cancel("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cancel: expected ErrNotFound, got %v", err)
	}
	// ByStatus filter works.
	_, _ = e.Distribute(context.Background(), approvedBlog(), DistributeRequest{Targets: []Platform{PlatformDevTo}})
	pubs, _ := e.Repo.ByStatus(StatusPublished)
	if len(pubs) != 1 {
		t.Errorf("ByStatus(Published) = %d, want 1", len(pubs))
	}
}

// --- helpers ---

func contentLookup(c Content) func(string) (Content, bool) {
	return func(id string) (Content, bool) {
		if id == c.ID {
			return c, true
		}
		return Content{}, false
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
