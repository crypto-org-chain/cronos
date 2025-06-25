# Architecture Decision Records (ADR)

This is a location to record all high-level architecture decisions in the Cronos implementation.

You can read more about the ADR concept in this [blog post](https://product.reverb.com/documenting-architecture-decisions-the-reverb-way-a3563bb24bd0#.78xhdix6t).

An ADR should provide:

- Context on the relevant goals and the current state
- Proposed changes to achieve the goals
- Summary of pros and cons
- References
- Changelog

Note the distinction between an ADR and a spec. The ADR provides the context, intuition, reasoning, and
justification for a change in architecture, or for the architecture of something
new. The spec is much more compressed and streamlined summary of everything as
it is or should be.

If recorded decisions turned out to be lacking, convene a discussion, record the new decisions here, and then modify the code to match.

Note the context/background should be written in the present tense.

To suggest an ADR, please make use of the [ADR template](./adr-template.md) provided.

## Table of Contents

| ADR \#               | Description                                                                | Status   |
|----------------------|----------------------------------------------------------------------------|----------|
| [001](./adr-001.md)  | Disable Gravity Bridge at Genesis                                          | Accepted |
| [002](./adr-002.md)  | Use a custom fork of ibc-go                                                | Accepted |
| [003](./adr-003.md)  | Add Fee Market Module                                                      | Accepted |
| [004](./adr-004.md)  | Tokens conversion in Cronos                                                | Accepted |
| [005](./adr-005.md)  | Cross-chain Validation for Gravity Bridge                                  | Rejected |
| [006](./adr-006.md)  | Migrating CRC20 contract to CRC21 standard                                 | Rejected |
| [007](./adr-007.md)  | Generic event format for evm-hook actions                                  | Accepted |
| [008](./adr-008.md)  | Denom and Contract Mapping Enhancement for Bi-Directional Token Conversion | Accepted |
| [009](./adr-009.md)  | Permissioned addresses in Cronos                                           | Accepted |
| [0010](./adr-010.md) | Custom precompiled for app-chain                                           | Accepted |
