# Exit codes

`zot` exits with one of four codes, so a script can tell "fix the command",
"start Zotero" and "Zotero said no" apart without parsing stderr. The codes are
the heibench table; zot uses the subset that applies to it.

| Code | Meaning | Examples |
|---|---|---|
| `0` | Done | The read returned, the write landed, `doctor` reports ready |
| `1` | Refused or failed | An object that does not exist; a write Zotero rejected; a batch, import or copy that came back `partial` or `failed`; Deny clicked in Zotero's authorize prompt; a write lease that does not cover this library or operation; the library changed since the write was prepared; invalid item JSON |
| `4` | Environment | Zotero not running or unreachable, or the connection dropped; the Local API turned off; a Zotero build with no write API; a missing or rejected key (`--web` without `ZOTGO_API_KEY`, a rejected API key or local key, no active `zot grant` lease or an expired one); Web API rate limiting; `doctor` reporting not ready |
| `64` | Usage | An unknown command, subcommand or flag; a missing, malformed or contradictory argument (`--json` with `--jsonl`, `--to` with `--to-top`); `--raw` on a command with no raw response; a write under `--web`; a machine-mode write without `--yes`; `zot grant` outside a terminal |

The line between `1` and `64`: `64` means the command line itself is wrong and
the same command will never work. `1` means the request was well formed and was
refused on its merits, including when what you supplied (item JSON, a file) is
not acceptable.

A `4` says nothing about your library or your request. Run `zot doctor` to see
which part of the environment is missing.

zot does not use `2` (could not tell) or `3` (ran, verified nothing).
