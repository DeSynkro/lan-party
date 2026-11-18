# Security policy

## Supported versions

The latest 1.x release receives security fixes. Pre-1.0 builds are not supported.

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Use GitHub's private vulnerability reporting:

https://github.com/DeSynkro/lan-party/security/advisories/new

Include:

- the LAN Party version and how you installed it (AppImage or source)
- steps to reproduce
- the impact you believe it has
- logs or a proof of concept, if you have them

If private reporting is unavailable to you, open a minimal public issue asking
for a private contact channel. Do not include the vulnerability details in that
issue.

## What to expect

- An acknowledgement within a few days.
- An initial assessment and a coordinated disclosure plan.
- Credit in the release notes if you want it.
- No bug bounty.

## Scope

In scope:

- the invite format, decoding and expiry
- device approval, identity and changed-key handling
- the QUIC tunnel, certificate pinning and control messages
- local TCP and UDP forwarding
- key storage and file permissions
- the AppImage and release artifacts
- dependency vulnerabilities that affect the shipped binary

Out of scope:

- a compromised host or friend machine
- router, ISP or network compromise
- physical access to either machine
- social engineering
- vulnerabilities in third-party software that do not affect LAN Party's use
  of it; report those upstream

Please only test on machines and networks you own or have explicit permission
to test.
