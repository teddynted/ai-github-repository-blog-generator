# Visual Assets: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

_acme/widget · release v1.0.0 · 14 assets_

## SDXL Visual System

_Target model: `stability-ai/sdxl` (via Replicate). Reuse these anchors across every asset instead of repeating style boilerplate._

### Shared SDXL Style Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.
```

### Shared SDXL Negative Prompt

```text
text, letters, numbers, words, logos, watermarks, signatures, clutter, excessive visual noise, distorted perspective, low resolution, blurry details, photorealistic humans, unrelated cloud services, busy background, tangled connectors, unreadable shapes
```

### Release Context

```yaml
release_context:
  repository: acme/widget
  release: v1.0.0
  feature: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS
  visual_theme: high engineering illustration
```

### Visual Variation

_Releases rotate composition archetypes so consecutive releases never repeat the same focal arrangement._

```yaml
visual_variation:
  active_archetype: diagonal event cascade
  archetypes:
    - right-weighted reveal
    - centered orchestration hub
    - diagonal event cascade
    - layered infrastructure stack
    - radial event burst
```

## Brand Guidelines

- **Primary colors:** #0B1F33, #12263A
- **Accent colors:** #FF9900, #4F9DFF
- **Illustration style:** flat vector with subtle isometric depth, clean geometric shapes, technical but friendly
- **Icon style:** minimal line + solid hybrid icons, consistent stroke weight, rounded corners
- **Background:** dark gradient with a faint grid or circuit texture, generous negative space
- **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows · **Depth:** layered flat planes with soft drop shadows for hierarchy
- **Typography placement:** headline zone reserved but left empty; strong left or lower-third alignment
- **Spacing:** generous margins, clear focal point, uncluttered composition
- **Visual tone:** modern, confident, engineering-credible, approachable

---

## Shared Render Constraints

_Applied to every asset below — referenced by each prompt, not repeated verbatim._

- No text, letters, numbers, logos, watermarks, or signatures
- Flat vector illustration with subtle isometric depth
- Soft directional key light with gentle rim highlights; no harsh shadows
- Layered flat planes with soft drop shadows for depth hierarchy
- Clean geometric shapes, consistent stroke weight, generous negative space
- Brand palette only: deep navy #0B1F33 / #12263A with amber #FF9900 and blue #4F9DFF accents

---

## YouTube Thumbnail

- **Platform:** YouTube · **Aspect ratio:** 16:9 (1280x720)
- **Purpose:** Drive clicks on the long-form release deep dive.
- **Recommended filename:** `widget-v1-0-0-youtube-thumbnail.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
```

### SDXL Parameters

```yaml
width: 1280
height: 720
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
- **Composition:** single bold focal subject offset to the right, large empty headline zone on the left third, strong visual hierarchy, minimal clutter
- **Perspective:** slight isometric hero angle for depth · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** high-energy, credible, click-worthy without being clickbait · **Technical focus:** release architecture at a glance
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** left third → headline; lower-left → release tag
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Platform Optimization

- One dominant focal object; extreme silhouette readability at 120px
- Strong warm/cool contrast with clear depth separation
- Avoid fine connector details that disappear on mobile

### Compact Prompt Variant

```text
event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration
```

### Automation Metadata

```yaml
asset_id: youtube_thumbnail
version: v1.0.0
theme: event_driven_architecture
render_priority: high
primary_use: video
supports_motion: true
```

### Motion Handoff

```yaml
motion_handoff:
  parallax_layers: 4
  animate_connectors: true
  animate_pulse_dots: true
  safe_crop_center: true
  preferred_zoom_anchor: orchestration_hub
```

---

## Repository Hero Image

- **Platform:** GitHub · **Aspect ratio:** 16:9 (1600x900)
- **Purpose:** Hero image for the repository README / landing.
- **Recommended filename:** `widget-v1-0-0-repository-hero-image.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
```

### SDXL Parameters

```yaml
width: 1600
height: 900
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
- **Composition:** wide banner composition with a central-to-right focal graphic and a generous headline zone, announcement-grade framing
- **Perspective:** hero isometric or gentle three-quarter view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** celebratory but professional, confident release energy · **Technical focus:** what the repository does
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** left third → announcement headline; lower-left → release tag
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Compact Prompt Variant

```text
event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration
```

### Automation Metadata

```yaml
asset_id: repository_hero_image
version: v1.0.0
theme: event_driven_architecture
render_priority: high
primary_use: repository
supports_motion: false
```

---

## GitHub Social Card

- **Platform:** GitHub · **Aspect ratio:** 1.91:1 (1280x640)
- **Purpose:** GitHub social preview when the repo is shared.
- **Recommended filename:** `widget-v1-0-0-github-social-card.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
```

### SDXL Parameters

```yaml
width: 1280
height: 640
scheduler: K_EULER
num_inference_steps: 30
guidance_scale: 7
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
- **Composition:** balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing
- **Perspective:** clean front or gentle isometric view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** professional, modern, trustworthy · **Technical focus:** widget v1.0.0
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** left half → headline; lower-left → repository name
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Platform Optimization

- Reads cleanly in GitHub dark-mode preview
- Legible when embedded in Slack, Discord, and X link previews
- Strong center-right focal cluster

### Automation Metadata

```yaml
asset_id: github_social_card
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: repository
supports_motion: true
```

### Motion Handoff

```yaml
motion_handoff:
  parallax_layers: 4
  animate_connectors: true
  animate_pulse_dots: true
  safe_crop_center: true
  preferred_zoom_anchor: orchestration_hub
```

---

## LinkedIn Banner

- **Platform:** LinkedIn · **Aspect ratio:** 1.91:1 (1200x627)
- **Purpose:** Professional announcement graphic for LinkedIn.
- **Recommended filename:** `widget-v1-0-0-linkedin-banner.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
```

### SDXL Parameters

```yaml
width: 1200
height: 627
scheduler: K_EULER
num_inference_steps: 30
guidance_scale: 7
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
- **Composition:** balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing
- **Perspective:** clean front or gentle isometric view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** professional, modern, trustworthy · **Technical focus:** release announcement
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** left half → headline; lower-left → repository name
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Platform Optimization

- Survives professional-feed compression on desktop and mobile
- Keep important detail out of the top-left profile-photo overlap area
- Respect desktop and mobile banner safe zones

### Automation Metadata

```yaml
asset_id: linkedin_banner
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: social
supports_motion: false
```

---

## X Image

- **Platform:** X · **Aspect ratio:** 16:9 (1600x900)
- **Purpose:** Shareable image for an X (Twitter) post.
- **Recommended filename:** `widget-v1-0-0-x-image.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: centered orchestration hub — put a central orchestration node with supporting planes radiating outward; reserve the lower band for overlay.
```

### SDXL Parameters

```yaml
width: 1600
height: 900
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** centered orchestration hub — put a central orchestration node with supporting planes radiating outward; reserve the lower band for overlay.
- **Composition:** balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing
- **Perspective:** clean front or gentle isometric view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** punchy, modern, shareable · **Technical focus:** release highlight
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** left half → headline; lower-left → repository name
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Automation Metadata

```yaml
asset_id: x_image
version: v1.0.0
theme: event_driven_architecture
render_priority: low
primary_use: social
supports_motion: true
```

### Motion Handoff

```yaml
motion_handoff:
  parallax_layers: 4
  animate_connectors: true
  animate_pulse_dots: true
  safe_crop_center: true
  preferred_zoom_anchor: orchestration_hub
```

---

## Blog Header

- **Platform:** Blog · **Aspect ratio:** 16:9 (1600x900)
- **Purpose:** Header image for the technical blog post.
- **Recommended filename:** `widget-v1-0-0-blog-header.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
```

### SDXL Parameters

```yaml
width: 1600
height: 900
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no stock-photo clichés, no photorealistic people
```

### Composition Notes

- **Archetype:** diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
- **Composition:** editorial header composition, a conceptual technical scene with a clear focal point and calm negative space for a title
- **Perspective:** gentle isometric or layered flat scene · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** thoughtful, technical, inviting · **Technical focus:** the technical story of the release
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** center or lower-third → article title
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Automation Metadata

```yaml
asset_id: blog_header
version: v1.0.0
theme: event_driven_architecture
render_priority: low
primary_use: social
supports_motion: false
```

---

## Dev.to Cover

- **Platform:** Dev.to · **Aspect ratio:** 1000:420 (1000x420)
- **Purpose:** Cover image for the Dev.to cross-post.
- **Recommended filename:** `widget-v1-0-0-dev-to-cover.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
```

### SDXL Parameters

```yaml
width: 1000
height: 420
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no stock-photo clichés, no photorealistic people
```

### Composition Notes

- **Archetype:** layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
- **Composition:** editorial header composition, a conceptual technical scene with a clear focal point and calm negative space for a title
- **Perspective:** gentle isometric or layered flat scene · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** thoughtful, technical, inviting · **Technical focus:** the technical story of the release
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** center or lower-third → article title
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Automation Metadata

```yaml
asset_id: dev_to_cover
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: social
supports_motion: false
```

---

## Medium Cover

- **Platform:** Medium · **Aspect ratio:** 3:2 (1500x1000)
- **Purpose:** Cover image for the Medium cross-post.
- **Recommended filename:** `widget-v1-0-0-medium-cover.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
```

### SDXL Parameters

```yaml
width: 1500
height: 1000
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no stock-photo clichés, no photorealistic people
```

### Composition Notes

- **Archetype:** radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
- **Composition:** editorial header composition, a conceptual technical scene with a clear focal point and calm negative space for a title
- **Perspective:** gentle isometric or layered flat scene · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** thoughtful, technical, inviting · **Technical focus:** the technical story of the release
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** center or lower-third → article title
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Automation Metadata

```yaml
asset_id: medium_cover
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: social
supports_motion: false
```

---

## Architecture Illustration

- **Platform:** Docs · **Aspect ratio:** 16:9 (1920x1080)
- **Purpose:** Explain the system architecture in documentation.
- **Recommended filename:** `widget-v1-0-0-architecture-illustration.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
```

### SDXL Parameters

```yaml
width: 1920
height: 1080
scheduler: K_DPM_2_ANCESTRAL
num_inference_steps: 40
guidance_scale: 8
high_noise_frac: 0.75
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no distorted diagrams, no tangled or crossing connectors, no unreadable component shapes, no unrelated AWS services (Amazon RDS, Amazon EKS, Amazon Redshift)
```

### Composition Notes

- **Archetype:** right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
- **Composition:** clear left-to-right technical diagram flow, labelled component blocks connected by directional arrows, balanced spacing, no text baked in
- **Perspective:** clean isometric or flat top-down schematic · **Lighting:** even, diagrammatic lighting with soft shadows for layering
- **Mood:** clear, educational, precise · **Technical focus:** component relationships
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** component blocks → node labels (added by compositor)
- **Grounded in:** release architecture diagram, AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** High · **Reliability:** 4/5 · **Best suited for:** GPT Image, Flux

### Diagrammatic Variant

```text
A documentation-first diagrammatic version of the same architecture: strict left-to-right flow, evenly spaced nodes with a clear directional arrow hierarchy, and minimal decorative elements. Leave each component block's interior empty and label-safe (render no text — interiors stay clean for labels added in compositing). Prioritise legibility over style: flat vector, brand palette, generous spacing, high contrast between nodes and background.
```

### Compact Prompt Variant

```text
event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration
```

### Automation Metadata

```yaml
asset_id: architecture_illustration
version: v1.0.0
theme: event_driven_architecture
render_priority: high
primary_use: docs
supports_motion: false
```

---

## AWS Workflow Diagram

- **Platform:** Docs · **Aspect ratio:** 16:9 (1920x1080)
- **Purpose:** Illustrate the AWS event flow.
- **Recommended filename:** `widget-v1-0-0-aws-workflow-diagram.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: centered orchestration hub — put a central orchestration node with supporting planes radiating outward; reserve the lower band for overlay.
```

### SDXL Parameters

```yaml
width: 1920
height: 1080
scheduler: K_DPM_2_ANCESTRAL
num_inference_steps: 40
guidance_scale: 8
high_noise_frac: 0.75
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no distorted diagrams, no tangled or crossing connectors, no unreadable component shapes, no unrelated AWS services (Amazon RDS, Amazon EKS, Amazon Redshift)
```

### Composition Notes

- **Archetype:** centered orchestration hub — put a central orchestration node with supporting planes radiating outward; reserve the lower band for overlay.
- **Composition:** clear left-to-right technical diagram flow, labelled component blocks connected by directional arrows, balanced spacing, no text baked in
- **Perspective:** clean isometric or flat top-down schematic · **Lighting:** even, diagrammatic lighting with soft shadows for layering
- **Mood:** clear, educational, precise · **Technical focus:** AWS service flow
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** component blocks → node labels (added by compositor)
- **Grounded in:** AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** High · **Reliability:** 4/5 · **Best suited for:** GPT Image, Flux

### Diagrammatic Variant

```text
A documentation-first diagrammatic version of the same architecture: strict left-to-right flow, evenly spaced nodes with a clear directional arrow hierarchy, and minimal decorative elements. Leave each component block's interior empty and label-safe (render no text — interiors stay clean for labels added in compositing). Prioritise legibility over style: flat vector, brand palette, generous spacing, high contrast between nodes and background.
```

### Compact Prompt Variant

```text
event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration
```

### Automation Metadata

```yaml
asset_id: aws_workflow_diagram
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: docs
supports_motion: false
```

---

## Release Card

- **Platform:** Generic · **Aspect ratio:** 1:1 (1080x1080)
- **Purpose:** Square release-announcement card for feeds.
- **Recommended filename:** `widget-v1-0-0-release-card.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
```

### SDXL Parameters

```yaml
width: 1080
height: 1080
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** diagonal event cascade — flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.
- **Composition:** centered square composition with a strong focal graphic, headline zone reserved along the lower third
- **Perspective:** hero isometric or gentle three-quarter view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** celebratory but professional, confident release energy · **Technical focus:** widget v1.0.0 release
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** lower third → release headline; top-left → logo
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Low · **Reliability:** 5/5 · **Best suited for:** GPT Image, Flux, Stable Diffusion

### Automation Metadata

```yaml
asset_id: release_card
version: v1.0.0
theme: event_driven_architecture
render_priority: low
primary_use: social
supports_motion: false
```

---

## Promotional Graphic

- **Platform:** Generic · **Aspect ratio:** 1:1 (1080x1080)
- **Purpose:** Promote the feature / open-source project.
- **Recommended filename:** `widget-v1-0-0-promotional-graphic.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
```

### SDXL Parameters

```yaml
width: 1080
height: 1080
scheduler: K_EULER
num_inference_steps: 35
guidance_scale: 7.5
high_noise_frac: 0.8
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** layered infrastructure stack — stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.
- **Composition:** centered square composition with a strong focal graphic, headline zone reserved along the lower third
- **Perspective:** hero isometric or gentle three-quarter view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** celebratory but professional, confident release energy · **Technical focus:** the headline feature
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** lower third → release headline; top-left → logo
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Compact Prompt Variant

```text
event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration
```

### Automation Metadata

```yaml
asset_id: promotional_graphic
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: social
supports_motion: false
```

---

## YouTube Shorts Cover

- **Platform:** YouTube Shorts · **Aspect ratio:** 9:16 (1080x1920)
- **Purpose:** Vertical cover for the YouTube Short.
- **Recommended filename:** `widget-v1-0-0-youtube-shorts-cover.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
```

### SDXL Parameters

```yaml
width: 1080
height: 1920
scheduler: K_EULER_ANCESTRAL
num_inference_steps: 32
guidance_scale: 7.8
high_noise_frac: 0.82
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** radial event burst — emit event paths radially from a single trigger; keep the outer margins clear for overlay text.
- **Composition:** balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing
- **Perspective:** clean front or gentle isometric view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** professional, modern, trustworthy · **Technical focus:** architecture Reveal
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** upper third → hook headline; lower third → handle / CTA
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Platform Optimization

- Keep the center 40% vertical band clear of platform UI overlays; avoid the right-edge interaction rail
- Large simple shapes; high-contrast focal cluster centered and readable at small preview sizes
- Reduced architectural complexity vs. desktop assets for small-screen viewing

### Automation Metadata

```yaml
asset_id: youtube_shorts_cover
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: video
supports_motion: true
```

### Motion Handoff

```yaml
motion_handoff:
  parallax_layers: 4
  animate_connectors: true
  animate_pulse_dots: true
  safe_crop_center: true
  preferred_zoom_anchor: orchestration_hub
```

---

## TikTok Cover

- **Platform:** TikTok · **Aspect ratio:** 9:16 (1080x1920)
- **Purpose:** Vertical cover for the TikTok video.
- **Recommended filename:** `widget-v1-0-0-tiktok-cover.png`
### SDXL Prompt

```text
Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks.

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
Composition: right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
```

### SDXL Parameters

```yaml
width: 1080
height: 1920
scheduler: K_EULER_ANCESTRAL
num_inference_steps: 32
guidance_scale: 7.8
high_noise_frac: 0.82
refine: expert_ensemble_refiner
```

### Prompt (grounded source)

```text
## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
```

### Negative Prompt

```text
no gibberish text, no misspelled words, no watermarks, no signatures, no logos, no clutter, no excessive visual noise, no distorted shapes, no low-resolution artifacts, no photorealistic human faces, no busy backgrounds that fight the headline
```

### Composition Notes

- **Archetype:** right-weighted reveal — place the focal cluster on the right; keep the left third empty for a headline overlay.
- **Composition:** balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing
- **Perspective:** clean front or gentle isometric view · **Lighting:** soft directional key light, gentle rim light on focal shapes, no harsh shadows
- **Mood:** professional, modern, trustworthy · **Technical focus:** architecture Insight
- **Palette:** #0B1F33, #12263A, #FF9900, #4F9DFF
- **Text placeholders (render no text):** upper third → hook headline; lower third → handle / CTA
- **Grounded in:** widget, v1.0.0, AWS Lambda, Amazon EventBridge, Amazon SQS

### Quality Checklist

- Single clear focal point
- Empty text-safe zone preserved
- No overlapping connector lines
- No tiny unreadable details
- Strong contrast between focal object and background
- Composition remains legible when scaled down

### Render Guidance

- **Complexity:** Medium · **Reliability:** 5/5 · **Best suited for:** GPT Image, Midjourney, Flux

### Platform Optimization

- Keep the center 40% vertical band clear of platform UI overlays; avoid the right-edge interaction rail
- Large simple shapes; high-contrast focal cluster centered and readable at small preview sizes
- Reduced architectural complexity vs. desktop assets for small-screen viewing

### Automation Metadata

```yaml
asset_id: tiktok_cover
version: v1.0.0
theme: event_driven_architecture
render_priority: medium
primary_use: video
supports_motion: true
```

### Motion Handoff

```yaml
motion_handoff:
  parallax_layers: 4
  animate_connectors: true
  animate_pulse_dots: true
  safe_crop_center: true
  preferred_zoom_anchor: orchestration_hub
```

---

## Collection Intelligence

- **Assets:** 14 · **Visual complexity:** high · **Est. cost:** ~16 credits total (provider-neutral estimate)
- **Audience:** Software engineers and cloud practitioners · **Difficulty:** intermediate
- **Platforms:** YouTube, GitHub, LinkedIn, X, Blog, Dev.to, Medium, Docs, Generic, YouTube Shorts, TikTok
- **SEO keywords:** AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, AWS CloudFormation, Amazon CloudWatch, Go, aws-lambda
- **Production notes:**
  - Prompts are provider-neutral: usable with GPT Image, DALL·E, Stable Diffusion, Midjourney, Nova Canvas, or Flux.
  - Images render no text — add real copy in the reserved placeholder zones during compositing.
  - Apply the shared Branding across every asset for a consistent visual identity.

---

## Collection Intelligence (machine-readable)

```yaml
collection_intelligence:
  assets: 14
  model_target: stability-ai/sdxl
  visual_complexity: high
  token_optimized: true
  automation_ready: true
  provider: replicate
  recommended_refiner: expert_ensemble_refiner
```

---

## AI Generation Rules

1. Reuse the shared SDXL style prompt as the anchor for every asset; add only the asset's focal subject.
2. Inject the release_context block dynamically (Step Functions / Lambda / n8n) — never hard-code the release.
3. Rotate the composition archetypes between releases; do not reuse the same focal arrangement consecutively.
4. Keep each asset's focal prompt concise (~70–140 words) — SDXL favours a clear subject over adjective stacking.
5. Preserve the reserved overlay zones so a downstream compositor can add real copy.
6. Generate images WITHOUT baked-in text, letters, logos, or watermarks (the shared negative prompt enforces this).
7. Feed the per-asset SDXL Parameters block straight to the Replicate stability-ai/sdxl API.

---

# Final Validation Matrix

| Asset | Text-safe zones | Mobile-safe | Docs-safe | Animation-ready |
| --- | :---: | :---: | :---: | :---: |
| YouTube Thumbnail | ✅ | ✅ | — | ✅ |
| Repository Hero Image | ✅ | ✅ | ✅ | — |
| GitHub Social Card | ✅ | ✅ | ✅ | ✅ |
| LinkedIn Banner | ✅ | ✅ | — | — |
| X Image | ✅ | ✅ | — | ✅ |
| Blog Header | ✅ | ✅ | ✅ | — |
| Dev.to Cover | ✅ | ✅ | ✅ | — |
| Medium Cover | ✅ | ✅ | ✅ | — |
| Architecture Illustration | ✅ | — | ✅ | — |
| AWS Workflow Diagram | ✅ | — | ✅ | — |
| Release Card | ✅ | ✅ | — | — |
| Promotional Graphic | ✅ | ✅ | — | — |
| YouTube Shorts Cover | ✅ | ✅ | — | ✅ |
| TikTok Cover | ✅ | ✅ | — | ✅ |

