package tui

import (
	"sort"
	"strings"
)

// buildModelOptions returns non-deprecated model IDs from provider docs (OpenAI
// "All models" + deprecations, Anthropic models overview, Google Gemini models).
// Image, video, embedding, TTS, realtime, audio, moderation, computer-use, and
// robotics models are omitted—they are not useful for this text-first data assistant
// flow. /model <id> still accepts any string.
func buildModelOptions() []string {
	curated := append(append(append([]string{}, openAICuratedModels()...), claudeCuratedModels()...), geminiCuratedModels()...)
	seen := make(map[string]struct{}, len(curated))
	out := make([]string, 0, len(curated))
	for _, m := range curated {
		model := strings.TrimSpace(m)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

// openAICuratedModels lists models from https://platform.openai.com/docs/models/all
// excluding entries explicitly tagged Deprecated there or on
// https://platform.openai.com/docs/deprecations (e.g. dall-e-2/3, o1-preview,
// o1-mini, codex-mini-latest, chatgpt-4o-latest, babbage-002, davinci-002).
func openAICuratedModels() []string {
	return []string{
		// Frontier & reasoning
		"gpt-5.4",
		"gpt-5.4-pro",
		"gpt-5.4-mini",
		"gpt-5.4-nano",
		"gpt-5-mini",
		"gpt-5-nano",
		"gpt-5",
		"gpt-5.2",
		"gpt-5.2-pro",
		"gpt-5.1",
		"gpt-5-pro",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o3",
		"o3-pro",
		"o3-mini",
		"o3-deep-research",
		"o4-mini",
		"o4-mini-deep-research",
		"o1",
		"o1-pro",
		// Codex
		"gpt-5-codex",
		"gpt-5-codex-mini",
		"gpt-5.3-codex",
		"gpt-5.2-codex",
		"gpt-5.1-codex",
		"gpt-5.1-codex-max",
		"gpt-5.1-codex-mini",
		// Open-weight
		"gpt-oss-120b",
		"gpt-oss-20b",
		// Web search (Chat Completions)
		"gpt-4o-search-preview",
		"gpt-4o-mini-search-preview",
		// ChatGPT-branded API snapshots (not recommended for all API use per docs)
		"gpt-5.3-chat-latest",
		"gpt-5.2-chat-latest",
		"gpt-5.1-chat-latest",
		"gpt-5-chat-latest",
	}
}

// claudeCuratedModels lists current Claude API IDs and aliases from
// https://docs.anthropic.com/en/docs/about-claude/models/overview
// Excludes claude-3-haiku-20240307 (deprecated per Anthropic).
func claudeCuratedModels() []string {
	return []string{
		// Claude 4.6 / 4.5 (latest)
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-5-20250929",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5",
		"claude-opus-4-5",
		// Claude 4.1 / 4.0
		"claude-opus-4-1-20250805",
		"claude-opus-4-1",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"claude-sonnet-4-0",
		"claude-opus-4-0",
		// Still common for tooling / migration
		"claude-3.5-haiku-20241022",
	}
}

// geminiCuratedModels lists models from https://ai.google.dev/gemini-api/docs/models
// excluding the "Previous models" / deprecated block (e.g. gemini-2.0-flash,
// gemini-3-pro-preview shut down). Omits image, video, embedding, audio/TTS,
// realtime, music, computer-use, and robotics models.
func geminiCuratedModels() []string {
	return []string{
		// Gemini 3 (text / multimodal)
		"gemini-3.1-pro-preview",
		"gemini-3-flash-preview",
		"gemini-3.1-flash-lite-preview",
		// Gemini 2.5 core
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		// Aliases (version patterns doc)
		"gemini-flash-latest",
		"gemini-pro-latest",
		// Deep Research (Interactions API model code per Gemini docs)
		"deep-research-pro-preview-12-2025",
	}
}
