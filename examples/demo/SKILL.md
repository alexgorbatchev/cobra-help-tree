---
name: mytool
description: Use when running mytool, the cobra-help-tree demonstration CLI, to manage users and API tokens or to read its help screens.
author: alexgorbatchev
metadata:
  created_on: 2026-10-08 09:21
  last_modified: 2026-10-08 10:45
  status: current
---

Run `mytool` with `AGENT=1`. A command then answers on stdout with a `command:` line and an `args:` line, and help is `key: value` text. Without `AGENT=1` a command answers with one `[OK] <command> <arguments>` line and help is a command tree.

`mytool` is a demonstration: every command reports its own invocation and changes nothing.

## Commands

- `mytool user create <name> [email]`: create a user. `<name>` is the login name and is required. `[email]` is the address invitations are sent to. `MYTOOL_DEFAULT_ROLE` names the role a new user starts with.
- `mytool user delete <id>`: remove the user with the numeric id `<id>`.
- `mytool user token issue <user-id>`: issue an API token that belongs to that user.
- `mytool user token revoke <token-id>`: invalidate that token at once.
- `mytool version`: takes no arguments.
- `mytool skill`: print this guide, byte for byte. Takes no arguments.
- `mytool help [command]`: print the help of a command, the screen `--help` prints for it.
- `mytool completion bash`, `mytool completion fish`, `mytool completion powershell`, `mytool completion zsh`: print a completion script for that shell on stdout. `--no-descriptions` (bool, default `false`) leaves the command descriptions out of the completions.

`mytool`, `mytool user`, `mytool user token`, and `mytool completion` are groups. Run one without a command to print its help, which lists everything below it.

## Flags

Every command accepts both.

- `--config <path>`, `-c <path>` (string, default `~/.config/mytool.yaml`): path to the configuration file.
- `--help`, `-h` (bool, default `false`): print the help of the command instead of running it.

## Errors

An unknown command or flag, a missing argument, or a surplus one ends the command with exit code 1. stderr carries an `Error:` line followed by the usage of the command that rejected the input.

## Examples

```bash
AGENT=1 mytool user create alice
AGENT=1 mytool user create bob bob@example.com
AGENT=1 mytool user token issue 42
AGENT=1 mytool user --help
```
