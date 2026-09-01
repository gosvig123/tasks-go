---
title: Completed task retention via gist-gated daily pruning
category: architecture
tags: [tasks, retention, gist, pruning, completed-tasks]
date: 2026-04-13
---

## Problem
Daily cleanup needed to remove stale completed tasks without dropping recent history or deleting tasks before they were backed up. The retention rule also had to fit the existing `today` list behavior and avoid breaking older completed task lines that never stored an `@done` timestamp.

## Solution
Keep `today` self-cleaning through its normal daily rebuild, and only prune source lists after a successful gist sync during daily refresh. Pruning removes completed tasks whose `@done` date is older than the retention cutoff, while completed tasks without `@done` stay in place so legacy data is preserved.

## Key Context
`today` is rebuilt from source references each day, so pruning it separately is unnecessary. Source lists are the durable records, so they are pruned only after gist backup succeeds. Retention depends on `@done`, not just `[x]`, because legacy completed tasks may not have a completion date.

## What Didn't Work
Pruning before gist sync risked deleting tasks that had not been backed up yet.
Pruning `today` added no value because daily reset already replaces its reference stubs.
Treating every `[x]` task as pruneable would delete legacy completed tasks without `@done`.
