# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from check_ai_coauthors import AI_EMAILS, ai_coauthors


SCRIPT = Path(__file__).with_name("check_ai_coauthors.py").resolve()
CLAUDE_TRAILER = "Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
SIGNOFF = "Signed-off-by: Example Contributor <contributor@example.com>"


class TrailerTests(unittest.TestCase):
    def test_known_ai_emails_are_rejected_regardless_of_display_name(self):
        emails = AI_EMAILS | {
            "175728472+Copilot@users.noreply.github.com",
            "123456+Claude@users.noreply.github.com",
        }
        for email in emails:
            with self.subTest(email=email):
                message = f"fix: example\n\nCo-authored-by: Any Name <{email}>\n{SIGNOFF}\n"
                self.assertEqual(list(ai_coauthors(message)), [email.lower()])

    def test_actual_model_name_and_dco_footer(self):
        message = f"fix: example\n\n{CLAUDE_TRAILER}\n{SIGNOFF}\n"
        self.assertEqual(list(ai_coauthors(message)), ["noreply@anthropic.com"])

    def test_case_crlf_and_multiple_coauthors(self):
        message = (
            "fix: example\r\n\r\n"
            "Co-authored-by: Claude Smith <claude@example.com>\r\n"
            "cO-AuThOrEd-bY: Assistant <NOREPLY@ANTHROPIC.COM>\r\n"
            "Co-authored-by: Helper <noreply@openai.com>\r\n"
        )
        self.assertEqual(list(ai_coauthors(message)), ["noreply@anthropic.com", "noreply@openai.com"])

    def test_humans_and_other_trailers_are_allowed(self):
        message = (
            "fix: improve Claude integration\n\n"
            "Co-authored-by: Claude Smith <claude@example.com>\n"
            "Co-authored-by: Devin Jones <devin@example.com>\n"
            "Co-authored-by: Employee <employee@anthropic.com>\n"
            "Co-authored-by: User <noreply@anthropic.com.example.org>\n"
            "Co-authored-by: dependabot[bot] <dependabot[bot]@users.noreply.github.com>\n"
            f"{SIGNOFF}\n"
            "AI-assisted-by: Claude <noreply@anthropic.com>\n"
        )
        self.assertEqual(list(ai_coauthors(message)), [])

    def test_quoted_examples_and_body_mentions_are_allowed(self):
        message = (
            "docs: explain attribution\n\n"
            f"Example:\n```text\n{CLAUDE_TRAILER}\n```\n\n"
            f"> {CLAUDE_TRAILER}\n\n{SIGNOFF}\n"
        )
        self.assertEqual(list(ai_coauthors(message)), [])


class RangeTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.env = dict(os.environ)
        for key in list(self.env):
            if key.startswith("GIT_"):
                del self.env[key]
        self.env.update(
            GIT_CONFIG_NOSYSTEM="1",
            GIT_CONFIG_GLOBAL=os.devnull,
            GIT_AUTHOR_NAME="Example Contributor",
            GIT_AUTHOR_EMAIL="contributor@example.com",
            GIT_COMMITTER_NAME="Example Contributor",
            GIT_COMMITTER_EMAIL="contributor@example.com",
        )
        self.git("init", "-q")
        self.base = self.commit("test: existing commit")

    def git(self, *args, input=None):
        return subprocess.run(
            ["git", *args], cwd=self.directory.name, env=self.env,
            input=input, text=True, capture_output=True, check=True,
        ).stdout.strip()

    def commit(self, message):
        self.git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-q", "-F", "-", input=message)
        return self.git("rev-parse", "HEAD")

    def check(self, base, head="HEAD"):
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--base", base, "--head", head],
            cwd=self.directory.name, env=self.env, text=True, capture_output=True,
        )

    def test_bad_earlier_commit_is_not_hidden_by_clean_head(self):
        bad = self.commit(f"fix: first\n\n{CLAUDE_TRAILER}\n{SIGNOFF}\n")
        self.commit(f"fix: second\n\n{SIGNOFF}\n")
        result = self.check(self.base)
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn(bad[:12], result.stdout)
        self.assertIn("Keep human co-authors", result.stdout)

    def test_existing_attribution_is_outside_the_incoming_range(self):
        old = self.commit(f"fix: existing\n\n{CLAUDE_TRAILER}\n")
        self.commit(f"fix: incoming\n\n{SIGNOFF}\n")
        result = self.check(old)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("1 commit(s)", result.stdout)

    def test_every_bad_commit_is_reported(self):
        bad = [self.commit(f"fix: change {i}\n\n{CLAUDE_TRAILER}\n") for i in range(2)]
        result = self.check(self.base)
        self.assertEqual(result.returncode, 1, result.stderr)
        for commit in bad:
            self.assertIn(commit[:12], result.stdout)

    def test_new_branch_push_checks_root_commit(self):
        self.git(
            "-c", "commit.gpgsign=false", "commit", "--amend", "--allow-empty", "-q", "-F", "-",
            input=f"test: root commit\n\n{CLAUDE_TRAILER}\n",
        )
        self.assertEqual(self.check("0" * 40).returncode, 1)

    def test_unknown_revision_fails_instead_of_passing_an_empty_range(self):
        for base, head in (("missing-ref", "HEAD"), (self.base, "missing-ref")):
            with self.subTest(base=base, head=head):
                result = self.check(base, head)
                self.assertEqual(result.returncode, 2)
                self.assertIn("Cannot check commit attribution", result.stderr)

    def test_diverged_branch_only_checks_incoming_commits(self):
        self.commit(f"fix: base advanced\n\n{CLAUDE_TRAILER}\n")
        advanced = self.git("rev-parse", "HEAD")
        self.git("checkout", "-q", "--detach", self.base)
        self.commit(f"fix: incoming\n\n{SIGNOFF}\n")
        result = self.check(advanced)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("1 commit(s)", result.stdout)


if __name__ == "__main__":
    unittest.main()
