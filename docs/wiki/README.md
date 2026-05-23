# `docs/wiki/`

These files are the **source of truth** for the project [Wiki][wiki]. The wiki itself is a separate git repository on GitHub, so we keep the canonical Markdown here (versioned alongside the code) and push it to the wiki on changes.

## Files

| Page in wiki              | File                          |
|---------------------------|-------------------------------|
| `Home`                    | [`Home.md`](./Home.md)        |
| `Getting Started`         | [`Getting-Started.md`](./Getting-Started.md) |
| `Architecture`            | [`Architecture.md`](./Architecture.md) |
| `Performance Notes`       | [`Performance-Notes.md`](./Performance-Notes.md) |
| `FAQ`                     | [`FAQ.md`](./FAQ.md)          |

## Pushing changes to the wiki

The wiki lives at `https://github.com/jdtoon/wachat.wiki.git` (a separate
git repo from this one). After editing files here:

```bash
# One-time clone of the wiki repo
git clone https://github.com/jdtoon/wachat.wiki.git /tmp/wachat-wiki

# Sync this directory's content into the wiki working copy
cp docs/wiki/*.md /tmp/wachat-wiki/

# Commit and push
cd /tmp/wachat-wiki
git add -A
git commit -m "Sync from docs/wiki/"
git push
```

[wiki]: https://github.com/jdtoon/wachat/wiki
