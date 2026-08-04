# Storyboard Scenes — per-scene SDXL image specs

`internal/storyboardscenes` (content-suite **Milestone 16**) converts the
generated storyboard into a canonical, Replicate `stability-ai/sdxl` **per-scene
visual specification** — the `storyboard-scenes.md` artifact. Each storyboard
scene becomes one independently renderable image spec, so a Fargate worker can
render a picture per scene and an FFmpeg pipeline can assemble short- and
long-form video.

It is a **deterministic transform** — no model call. Every field is derived from
the storyboard's own scenes; it invents no architecture.

## Per-scene fields

| Field | Derived from |
| --- | --- |
| **Purpose** | scene `type` → narrative purpose (Hook, Problem, Architecture, …) |
| **Narration Alignment** | the scene's first narration sentence |
| **Camera Direction** | scene `camera.direction`, normalized to an FFmpeg-friendly vocabulary (slow push in, left pan, orbit move, …) |
| **SDXL Prompt** | shared brand style anchor + focal subject (the scene's `visuals.description`, or a grounded metaphor for its type when the visual is text/logo-centric) + a rotated cinematic composition |
| **Negative Prompt** | the shared negative prompt (no text, logos, watermarks, …) |
| **Motion Suggestion** | the scene's first animation cue, or its type (Ken Burns, node pulse, event flow, …) |
| **Duration** | the scene's recommended seconds |

## Composition rotation

Seven cinematic compositions (`centered orchestration hub`, `right-weighted
reveal`, `diagonal event cascade`, `layered infrastructure stack`, `radial event
burst`, `observability control room`, `immutable infrastructure pipeline`) are
rotated per release + scene index, so consecutive scenes stay visually distinct.

## Shared with the renderer

`storyboardscenes.ScenePrompt` and `ChooseComposition` are the **single source of
truth** for the scene prompt: the video renderer builds its per-scene image
prompt from the same functions, so a scene's rendered image matches its spec. See
[Social Video Generation](./social-video.md).
