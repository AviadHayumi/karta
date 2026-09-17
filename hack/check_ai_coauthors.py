# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

"""Reject known AI identities in the Git co-author trailers of incoming commits."""

import argparse
import re
import subprocess
import sys


AI_EMAILS = frozenset(
    {
        "noreply@anthropic.com",
        "noreply@openai.com",
        "gemini-code-assist@google.com",
        "cursoragent@cursor.com",
    }
)
AI_GITHUB_EMAIL = re.compile(
    r"\d+\+(?:claude|copilot)@users\.noreply\.github\.com", re.IGNORECASE
)


def git(*args, input=None):
    return subprocess.run(
        ["git", *args],
        input=input,
        text=True,
        encoding="utf-8",
        errors="replace",
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    ).stdout.strip()


def ai_coauthors(message):
    trailers = git("-c", "trailer.separators=:", "interpret-trailers", "--parse", input=message)
    for trailer in trailers.splitlines():
        key, separator, value = trailer.partition(":")
        if not separator or key.strip().lower() != "co-authored-by":
            continue
        identity = re.fullmatch(r"[^<>]*<([^<>\s]+)>\s*", value)
        if identity:
            email = identity.group(1).lower()
            if email in AI_EMAILS or AI_GITHUB_EMAIL.fullmatch(email):
                yield email


def check_commits(base, head):
    head = git("rev-parse", "--verify", "--end-of-options", f"{head}^{{commit}}")
    # A new branch push has no previous tip. Check its complete reachable history.
    if base == "0" * 40:
        revision_range = head
    else:
        base = git("rev-parse", "--verify", "--end-of-options", f"{base}^{{commit}}")
        revision_range = f"{base}..{head}"
    commits = git("rev-list", "--reverse", revision_range).splitlines()
    rejected = []
    for commit in commits:
        message = git("show", "-s", "--format=%B", commit)
        for email in ai_coauthors(message):
            rejected.append((commit, email))
    return commits, rejected


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True, help="Exclusive base commit or ref")
    parser.add_argument("--head", required=True, help="Inclusive head commit or ref")
    args = parser.parse_args()
    try:
        commits, rejected = check_commits(args.base, args.head)
    except (OSError, subprocess.CalledProcessError) as error:
        print(f"Cannot check commit attribution: {error}", file=sys.stderr)
        return 2
    if rejected:
        for commit, email in rejected:
            print(f"{commit[:12]}: AI Co-authored-by identity <{email}> is not allowed.")
        print("Remove the AI Co-authored-by trailer from each listed commit.")
        print("Keep human co-authors and the required human Signed-off-by trailer.")
        return 1
    print(f"Commit attribution passed for {len(commits)} commit(s).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
