# FINDEROAD — notes for Claude

**Read `PROJECT_NOTES.md` first** (structure, decisions, progress checklist) and update it at the end of every task.
FINDEROAD is a personal macOS (Apple Silicon) file search / preview / copy / move / delete app for **Nevertheless_D**,
who is not a programmer. Talk to them in Korean, in plain words. Never ask them to use Terminal.
Never mention GitHub (or the repo name) in anything the user sees (app screens, guide image).

Architecture (same as MSROAD 1.1.0+): FINDEROAD.app is a real, visible Dock app — a Swift shell (`app/shell/main.swift`)
with a WKWebView window. The Go engine (`engine/`) runs as the app's direct child only while the app is open and exits
when the app goes away. Never add hidden helper apps, SIGTERM-ignoring or launchers that leave the engine running.

The app can only be built on a Mac: pushing changes under app/, engine/, scripts/ or VERSION triggers
`.github/workflows/build-mac.yml`, which builds, ad-hoc signs, runs real-Mac smoke tests and force-pushes results to the
`ci-results` branch (read results via git: the Actions API is blocked in Claude's sessions). Release with `scripts/release.sh`.
Commit often (the user asked for checkpoints so work can resume after an interruption).
In tests use `pkill -x finderoad-engine`, never `pkill -f` (it matches your own shell).
