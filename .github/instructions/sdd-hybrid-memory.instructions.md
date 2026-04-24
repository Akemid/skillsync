---
description: "Use when running SDD workflows or commands like /sdd-new, /sdd-continue, /sdd-ff, /sdd-explore, /sdd-apply, /sdd-verify, /sdd-archive. Enforce hybrid artifact mode (Engram + OpenSpec) and memory lookup across both project names: uniqueskills and skillsync."
name: "SDD Hybrid Memory Policy"
---
# SDD Hybrid Memory Policy

When executing any SDD workflow in this repository:

- Force artifact mode to `hybrid` unless the user explicitly overrides it.
- Persist every SDD phase artifact in both backends:
- Engram topic key format: `sdd/{change-name}/{artifact}`
- OpenSpec path format: `openspec/changes/{change-name}/{artifact}.md`
- Before starting an SDD phase, search memory context in both project identifiers:
- First search with project `uniqueskills`
- Then search with project `skillsync`
- If either search returns relevant context, use it and cite which project key was used.
- If context appears in both, prefer the most recent artifact and keep topic-key continuity for future writes.

Operational guardrails:

- Never skip Engram persistence when OpenSpec write succeeds.
- Never skip OpenSpec persistence when Engram save succeeds.
- On retrieval, do not rely on truncated search previews; retrieve full artifact content before decisions.
- If one backend is temporarily unavailable, continue with the other and report the partial persistence explicitly.
