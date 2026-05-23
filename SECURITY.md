# Security Policy

## Reporting a vulnerability

Please **do not** open public GitHub issues for security problems.

Email **junaidd0306@gmail.com** with:

- A clear description of the issue
- Steps to reproduce, ideally with a minimal repro
- The affected commit/tag

You will get an acknowledgement within a few days. Once a fix is ready we will
coordinate a disclosure window with you.

## Scope and limitations

`wachat` is a personal, unofficial WhatsApp client built on
[`whatsmeow`](https://pkg.go.dev/go.mau.fi/whatsmeow), which is a
reverse-engineered library. Two things follow from that:

1. **Account-ban risk is intrinsic.** Using `wachat` violates WhatsApp's Terms
   of Service. We mitigate by behaving like a human client and persisting the
   session properly, but we cannot eliminate the risk. This is documented and
   accepted scope; reports of "WhatsApp can ban this account" are not
   security issues.
2. **Cryptography is delegated to `whatsmeow`.** Vulnerabilities in the
   Signal-protocol implementation belong upstream. Please report them at
   <https://github.com/tulir/whatsmeow>. We will track upstream advisories and
   bump the dependency as fixes land.

In-scope for this repo's security policy:

- Bugs that cause the local SQLite store, session data, or media files to be
  read or modified by an unprivileged process on the same machine.
- Bugs that cause `wachat` to leak credentials, session tokens, or message
  contents to disk in plaintext outside the intended store.
- Bugs in input handling that allow a remote peer to crash the client or
  trigger memory exhaustion.
