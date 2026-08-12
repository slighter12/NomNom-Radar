# Domain Docs

How the engineering skills should consume this repo's domain documentation
when exploring the codebase.

## Before exploring, read these

- `CONTEXT.md` at the repository root.
- `docs/adr/` entries relevant to the area being changed.

If these files do not exist, proceed silently. Do not create empty placeholders.
`/domain-modeling` creates them lazily when real terminology or durable
decisions need to be recorded and the request authorizes documentation
changes.

## Layout

The repository uses a single-context target layout. `CONTEXT.md` and
`docs/adr/` are created lazily by `/domain-modeling`; their absence means that
no glossary or ADR has needed to be recorded yet.

```
/
├── CONTEXT.md
├── docs/adr/
└── internal/
```

## Vocabulary

Use domain terms as defined in `CONTEXT.md`. Do not drift to synonyms that the
glossary explicitly avoids.

If a required concept is missing, reconsider whether the term belongs to the
project or record the gap for `/domain-modeling`.

## ADR conflicts

If proposed work contradicts an existing ADR, surface the conflict explicitly
instead of silently overriding the decision.
