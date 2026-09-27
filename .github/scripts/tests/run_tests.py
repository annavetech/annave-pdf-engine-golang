#!/usr/bin/env python3
"""pdfengine .github/scripts test suite.
Covers the repo-automation scripts only: label/status logic, chat-ops authorization, label sync."""

import json
import os
import re
import sys
import tempfile
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPTS_DIR = os.path.dirname(HERE)
GITHUB_DIR = os.path.dirname(SCRIPTS_DIR)
ROOT = os.path.dirname(GITHUB_DIR)
sys.path.insert(0, SCRIPTS_DIR)

import auto_assign  # noqa: E402
import chat_commands  # noqa: E402
import needs_ok_to_test  # noqa: E402
import size_label  # noqa: E402
import strip_stale_approval  # noqa: E402
import sync_labels  # noqa: E402
from merge_gate import MAX_DESCRIPTION_LENGTH, decide_state  # noqa: E402

PASS, FAIL = 0, 0

# No automation script may ever print an emoji.
EMOJI_PATTERN = re.compile(
    "["
    "\U0001F300-\U0001FAFF"  # misc symbols and pictographs through extended-A
    "\U00002600-\U000027BF"  # misc symbols, dingbats
    "\U0000FE0F"  # variation selector-16 (emoji presentation)
    "]"
)

TEST_REPO = "annavetech/pdfengine"


def ok(name):
    global PASS
    PASS += 1
    print(f"ok   {name}")


def bad(name, detail):
    global FAIL
    FAIL += 1
    print(f"FAIL {name}")
    print(f"     {detail}")


# --- merge_gate.decide_state ---

def test_merge_gate_decide_state():
    cases = [
        (set(), "pending", "Needs approved, lgtm labels"),
        ({"lgtm"}, "pending", "Needs approved label"),
        ({"approved"}, "pending", "Needs lgtm label"),
        ({"lgtm", "approved"}, "success", "lgtm and approved; not held"),
        ({"lgtm", "approved", "do-not-merge/hold"}, "pending", "Blocked by do-not-merge/hold"),
        ({"do-not-merge/hold"}, "pending", "Blocked by do-not-merge/hold"),
        (
            {"lgtm", "do-not-merge/hold", "do-not-merge/work-in-progress"},
            "pending",
            "Blocked by do-not-merge/hold, do-not-merge/work-in-progress",
        ),
    ]
    for labels, expect_state, expect_desc in cases:
        state, desc = decide_state(labels)
        name = f"decide_state({sorted(labels)!r}) == ({expect_state!r}, {expect_desc!r})"
        if state == expect_state and desc == expect_desc:
            ok(name)
        else:
            bad(name, f"got ({state!r}, {desc!r})")


def test_merge_gate_description_length_capped():
    # Not a realistic input; confirms an unusual pile-up of hold labels stays within the limit.
    holds = {f"do-not-merge/a-fairly-long-reason-{i}" for i in range(20)}
    state, desc = decide_state(holds)
    name = "decide_state caps description at GitHub's 140-character limit"
    if state == "pending" and len(desc) <= MAX_DESCRIPTION_LENGTH:
        ok(name)
    else:
        bad(name, f"len={len(desc)} desc={desc!r}")


# --- chat_commands.parse_command ---

def test_parse_command_recognizes_first_nonblank_line():
    cases = [
        ("/lgtm", "/lgtm"),
        ("\n\n/approve\nthanks", "/approve"),
        ("not a command", None),
        ("", None),
        (None, None),
    ]
    for body, expect in cases:
        got = chat_commands.parse_command(body)
        name = f"parse_command({body!r}) == {expect!r}"
        if got == expect:
            ok(name)
        else:
            bad(name, f"got {got!r}")


# --- chat_commands handler authorization branches ---

def test_handle_lgtm_rejects_unauthorized_actor():
    with mock.patch("chat_commands.is_authorized", return_value=False), \
            mock.patch("chat_commands.post_comment", return_value=(201, {})) as comment, \
            mock.patch("chat_commands.add_labels") as add_labels:
        chat_commands.handle_lgtm(TEST_REPO, 1, "outsider")
    name = "handle_lgtm posts a rejection comment and never labels when the actor lacks access"
    if comment.called and not add_labels.called:
        ok(name)
    else:
        bad(name, f"comment.called={comment.called} add_labels.called={add_labels.called}")


def test_handle_lgtm_labels_authorized_actor():
    with mock.patch("chat_commands.is_authorized", return_value=True), \
            mock.patch("chat_commands.add_labels", return_value=(200, {})) as add_labels, \
            mock.patch("chat_commands.refresh_merge_gate_status"):
        chat_commands.handle_lgtm(TEST_REPO, 1, "maintainer")
    name = "handle_lgtm adds the lgtm label for an authorized actor"
    if add_labels.call_args == mock.call(TEST_REPO, 1, ["lgtm"]):
        ok(name)
    else:
        bad(name, f"add_labels call: {add_labels.call_args}")


def test_handle_hold_allows_pr_author_without_write_access():
    # /hold and /unhold trust the PR's own author in addition to a maintainer.
    with mock.patch("chat_commands.is_authorized", return_value=False), \
            mock.patch("chat_commands.add_labels", return_value=(200, {})) as add_labels, \
            mock.patch("chat_commands.refresh_merge_gate_status"):
        chat_commands.handle_hold(TEST_REPO, 1, "contributor", "contributor")
    name = "handle_hold allows the PR's own author even without write access"
    if add_labels.call_args == mock.call(TEST_REPO, 1, ["do-not-merge/hold"]):
        ok(name)
    else:
        bad(name, f"add_labels call: {add_labels.call_args}")


def test_handle_ok_to_test_never_trusts_the_pr_author():
    # Unlike /hold, the PR's own author must never authorize their own CI run.
    with mock.patch("chat_commands.is_authorized", return_value=False), \
            mock.patch("chat_commands.post_comment", return_value=(201, {})) as comment, \
            mock.patch("chat_commands.remove_label") as remove_label, \
            mock.patch("chat_commands.add_labels") as add_labels:
        chat_commands.handle_ok_to_test(TEST_REPO, 1, "contributor", "contributor")
    name = "handle_ok_to_test rejects the PR's own author, unlike /hold"
    if comment.called and not remove_label.called and not add_labels.called:
        ok(name)
    else:
        bad(name, f"comment={comment.called} remove={remove_label.called} add={add_labels.called}")


# --- strip_stale_approval.strip_and_report ---

def test_strip_stale_approval_removes_lgtm_and_approved():
    with (
        mock.patch("strip_stale_approval.remove_label", return_value=(200, None)) as remove,
        mock.patch("strip_stale_approval.set_commit_status", return_value=(201, {})) as set_status,
    ):
        result = strip_stale_approval.strip_and_report(TEST_REPO, 1, "sha1", {"lgtm", "approved"})
    name = "strip_and_report removes both lgtm and approved when present"
    removed = sorted(c.args[2] for c in remove.call_args_list)
    expect_status = mock.call(TEST_REPO, "sha1", "pending", "Needs approved, lgtm labels")
    if result and removed == ["approved", "lgtm"] and set_status.call_args == expect_status:
        ok(name)
    else:
        bad(name, f"result={result} removed={removed} set_status={set_status.call_args}")


def test_strip_stale_approval_leaves_hold_untouched():
    with (
        mock.patch("strip_stale_approval.remove_label", return_value=(200, None)) as remove,
        mock.patch("strip_stale_approval.set_commit_status", return_value=(201, {})) as set_status,
    ):
        result = strip_stale_approval.strip_and_report(
            TEST_REPO, 1, "sha1", {"lgtm", "approved", "do-not-merge/hold"}
        )
    name = "strip_and_report never removes a do-not-merge/* hold label"
    removed = [c.args[2] for c in remove.call_args_list]
    expect_status = mock.call(TEST_REPO, "sha1", "pending", "Blocked by do-not-merge/hold")
    if result and "do-not-merge/hold" not in removed and set_status.call_args == expect_status:
        ok(name)
    else:
        bad(name, f"removed={removed} set_status={set_status.call_args}")


def test_strip_stale_approval_fails_closed_on_remove_error():
    with mock.patch("strip_stale_approval.remove_label", return_value=(403, {"message": "no"})), \
            mock.patch("strip_stale_approval.set_commit_status") as set_status:
        result = strip_stale_approval.strip_and_report(TEST_REPO, 1, "sha1", {"lgtm"})
    name = "strip_and_report fails closed and posts nothing when a label removal errors"
    if result is False and not set_status.called:
        ok(name)
    else:
        bad(name, f"result={result} set_status.called={set_status.called}")


# --- needs_ok_to_test ---

def test_needs_ok_to_test_labels_unauthorized_author():
    event = {"pull_request": {"number": 1, "user": {"login": "outsider"}}}
    fd, event_path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(event, f)
    try:
        with mock.patch.dict(
            os.environ,
            {"GITHUB_EVENT_PATH": event_path, "GITHUB_REPOSITORY": TEST_REPO},
        ), mock.patch("needs_ok_to_test.is_authorized", return_value=False), \
                mock.patch("needs_ok_to_test.add_labels", return_value=(200, {})) as add_labels:
            needs_ok_to_test.main()
    finally:
        os.remove(event_path)
    name = "needs-ok-to-test labels a PR opened by an unauthorized author"
    if add_labels.call_args == mock.call(TEST_REPO, 1, ["needs-ok-to-test"]):
        ok(name)
    else:
        bad(name, f"add_labels call: {add_labels.call_args}")


def test_needs_ok_to_test_skips_authorized_author():
    event = {"pull_request": {"number": 1, "user": {"login": "maintainer"}}}
    fd, event_path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(event, f)
    try:
        with mock.patch.dict(
            os.environ,
            {"GITHUB_EVENT_PATH": event_path, "GITHUB_REPOSITORY": TEST_REPO},
        ), mock.patch("needs_ok_to_test.is_authorized", return_value=True), \
                mock.patch("needs_ok_to_test.add_labels") as add_labels:
            needs_ok_to_test.main()
    finally:
        os.remove(event_path)
    name = "needs-ok-to-test never labels a PR opened by an authorized author"
    if not add_labels.called:
        ok(name)
    else:
        bad(name, "add_labels was called for an authorized author")


# --- size_label.bucket_for ---

def test_size_label_bucket_thresholds():
    cases = [
        (0, "size/XS"),
        (9, "size/XS"),
        (10, "size/S"),
        (29, "size/S"),
        (30, "size/M"),
        (99, "size/M"),
        (100, "size/L"),
        (499, "size/L"),
        (500, "size/XL"),
        (999, "size/XL"),
        (1000, "size/XXL"),
        (5000, "size/XXL"),
    ]
    for lines, expect in cases:
        got = size_label.bucket_for(lines)
        name = f"bucket_for({lines}) == {expect!r}"
        if got == expect:
            ok(name)
        else:
            bad(name, f"got {got!r}")


# --- auto_assign: issue vs PR dispatch ---

def test_auto_assign_issue_assigns_maintainer():
    event = {"issue": {"number": 7}}
    fd, event_path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(event, f)
    try:
        with mock.patch.dict(
            os.environ,
            {
                "GITHUB_EVENT_PATH": event_path,
                "GITHUB_REPOSITORY": TEST_REPO,
                "GITHUB_EVENT_NAME": "issues",
                "MAINTAINER_LOGIN": "maintainer",
            },
        ), mock.patch("auto_assign.add_assignees", return_value=(201, {})) as add_assignees:
            auto_assign.main()
    finally:
        os.remove(event_path)
    name = "auto-assign assigns an opened issue to MAINTAINER_LOGIN"
    if add_assignees.call_args == mock.call(TEST_REPO, 7, ["maintainer"]):
        ok(name)
    else:
        bad(name, f"add_assignees call: {add_assignees.call_args}")


def test_auto_assign_pr_assigns_its_own_author():
    event = {"pull_request": {"number": 8, "user": {"login": "contributor"}}}
    fd, event_path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(event, f)
    try:
        with mock.patch.dict(
            os.environ,
            {
                "GITHUB_EVENT_PATH": event_path,
                "GITHUB_REPOSITORY": TEST_REPO,
                "GITHUB_EVENT_NAME": "pull_request_target",
            },
        ), mock.patch("auto_assign.add_assignees", return_value=(201, {})) as add_assignees:
            auto_assign.main()
    finally:
        os.remove(event_path)
    name = "auto-assign assigns an opened PR to its own author"
    if add_assignees.call_args == mock.call(TEST_REPO, 8, ["contributor"]):
        ok(name)
    else:
        bad(name, f"add_assignees call: {add_assignees.call_args}")


# --- sync_labels: create / update / delete diff ---

def test_sync_labels_creates_missing_and_updates_mismatched():
    existing = {
        "kind/bug": {"color": "old-color", "description": "stale"},
    }
    desired = {
        "kind/bug": {"color": "d73a4a", "description": "fresh"},
        "kind/enhancement": {"color": "a2eeef", "description": "new"},
    }
    calls = []

    def fake_request(method, path, body=None):
        calls.append((method, path, body))
        if method == "POST":
            return 201, {}
        if method == "PATCH":
            return 200, {}
        return 200, {}

    with mock.patch("sync_labels.list_existing_labels", return_value=existing), \
            mock.patch("sync_labels.load_desired", return_value=desired), \
            mock.patch("sync_labels.github_request", side_effect=fake_request), \
            mock.patch.dict(os.environ, {"GITHUB_REPOSITORY": TEST_REPO}):
        rc = sync_labels.main()
    methods = [c[0] for c in calls]
    name = "sync_labels creates a missing label and updates a mismatched one"
    if rc == 0 and "POST" in methods and "PATCH" in methods:
        ok(name)
    else:
        bad(name, f"rc={rc} calls={calls}")


def test_sync_labels_deletes_labels_not_in_desired():
    existing = {
        "kind/bug": {"color": "d73a4a", "description": "fresh"},
        "obsolete": {"color": "ffffff", "description": "gone"},
    }
    desired = {
        "kind/bug": {"color": "d73a4a", "description": "fresh"},
    }
    calls = []

    def fake_request(method, path, body=None):
        calls.append((method, path, body))
        return 204, None

    with mock.patch("sync_labels.list_existing_labels", return_value=existing), \
            mock.patch("sync_labels.load_desired", return_value=desired), \
            mock.patch("sync_labels.github_request", side_effect=fake_request), \
            mock.patch.dict(os.environ, {"GITHUB_REPOSITORY": TEST_REPO}):
        rc = sync_labels.main()
    name = "sync_labels deletes a label no longer in labels.json"
    deletes = [c for c in calls if c[0] == "DELETE" and "obsolete" in c[1]]
    if rc == 0 and deletes:
        ok(name)
    else:
        bad(name, f"rc={rc} calls={calls}")


def test_sync_labels_reports_failure_on_bad_status():
    existing = {}
    desired = {"kind/bug": {"color": "d73a4a", "description": "fresh"}}

    with mock.patch("sync_labels.list_existing_labels", return_value=existing), \
            mock.patch("sync_labels.load_desired", return_value=desired), \
            mock.patch("sync_labels.github_request", return_value=(500, {"message": "no"})), \
            mock.patch.dict(os.environ, {"GITHUB_REPOSITORY": TEST_REPO}):
        rc = sync_labels.main()
    name = "sync_labels returns non-zero when a create/update/delete call fails"
    if rc == 1:
        ok(name)
    else:
        bad(name, f"rc={rc}")


# --- no emoji, ever ---

def test_no_emoji_under_github_scripts():
    offenders = []
    for dirpath, _dirnames, filenames in os.walk(SCRIPTS_DIR):
        if "__pycache__" in dirpath:
            continue
        for filename in filenames:
            path = os.path.join(dirpath, filename)
            try:
                with open(path, encoding="utf-8") as f:
                    text = f.read()
            except (UnicodeDecodeError, OSError):
                continue
            if EMOJI_PATTERN.search(text):
                offenders.append(os.path.relpath(path, ROOT))
    name = "no emoji under .github/scripts"
    if offenders:
        bad(name, f"emoji found in: {offenders}")
    else:
        ok(name)


def main():
    for name, func in sorted(globals().items()):
        if name.startswith("test_") and callable(func):
            func()

    print(f"\n{PASS} passed, {FAIL} failed")
    return 1 if FAIL else 0


if __name__ == "__main__":
    sys.exit(main())
