#!/usr/bin/env python3
"""Prints release notes for the commits since the last vX.Y.Z tag, grouped by Conventional Commit
type: breaking changes, features, fixes. Other types (docs, chore, ci, test, refactor) are left out.

usage: release-notes.py NEW_TAG   (GITHUB_REPOSITORY, if set, adds a compare link)
"""
import os
import re
import subprocess
import sys

SUBJECT = re.compile(r"^(?P<type>[a-z]+)(\([^)]*\))?(?P<bang>!)?: (?P<desc>.+)$")
MERGE = re.compile(r"^Merge pull request #(\d+)")
SQUASH = re.compile(r"\(#(\d+)\)$")


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def last_tag():
    r = subprocess.run(["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*.[0-9]*.[0-9]*"],
                       capture_output=True, text=True)
    return r.stdout.strip() if r.returncode == 0 else ""


def commits(rng):
    """(subject, body) of each non-merge commit in rng."""
    out = git("log", "--no-merges", "--reverse", "--format=%s%x1f%b%x1e", *rng)
    return [tuple(c.strip("\n").split("\x1f", 1)) for c in out.split("\x1e") if c.strip()]


def changes(since):
    """(subject, body, pr) for every commit since the tag, with the PR that brought it in."""
    rng = [f"{since}..HEAD"] if since else ["HEAD"]
    out = []
    for line in git("log", "--first-parent", "--reverse", "--format=%H %P%x1f%s", *rng).splitlines():
        shas, subject = line.split("\x1f", 1)
        sha, *parents = shas.split()
        m = MERGE.match(subject)
        if len(parents) == 2:
            pr = m.group(1) if m else ""
            out += [(s, b, pr) for s, b in commits([f"{parents[0]}..{parents[1]}"])]
        else:
            s, b = commits([f"{sha}^!"] if parents else [sha, "-1"])[0]
            sq = SQUASH.search(s)
            out.append((SQUASH.sub("", s).rstrip(), b, sq.group(1) if sq else ""))
    return out


def notes(tag, since, repo):
    groups = {"Breaking changes": [], "Features": [], "Fixes": []}
    for subject, body, pr in changes(since):
        m = SUBJECT.match(subject)
        if not m:
            continue
        line = "- " + m.group("desc") + (f" (#{pr})" if pr else "")
        breaking = m.group("bang") or re.search(r"^BREAKING[ -]CHANGE:", body, re.M)
        if breaking:
            groups["Breaking changes"].append(line)
        elif m.group("type") == "feat":
            groups["Features"].append(line)
        elif m.group("type") in ("fix", "perf"):
            groups["Fixes"].append(line)
    parts = [f"### {name}\n\n" + "\n".join(lines) for name, lines in groups.items() if lines]
    if repo and since:
        parts.append(f"**Full changelog:** https://github.com/{repo}/compare/{since}...{tag}")
    return "\n\n".join(parts) + "\n"


if __name__ == "__main__":
    sys.stdout.write(notes(sys.argv[1], last_tag(), os.environ.get("GITHUB_REPOSITORY", "")))
