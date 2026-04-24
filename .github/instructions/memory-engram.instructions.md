---
description: "Use when the user mentions memory, recuerdos, remember, mem, or asks to save/retrieve context. Treat memory operations as Engram MCP by default."
name: "Memory Means Engram MCP"
---
# Memory Means Engram MCP

When the user says memory (or synonyms such as memoria, recuerdos, remember, mem):

- Interpret it as Engram MCP, not local filesystem memory.
- Prefer Engram tools for persistence and retrieval.
- If a prior response used local /memories storage, correct course and save in Engram MCP.
- Explicitly state when an operation was saved to Engram MCP.
- Only use local /memories if the user explicitly requests local memory files.

Operational policy:

- For retrieval, search in Engram first.
- For persistence, write to Engram first.
- If Engram is unavailable, report that clearly and ask whether to fallback to local /memories.
