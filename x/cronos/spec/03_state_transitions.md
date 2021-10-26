<!--
order: 3
-->

# State Transitions

<!-- define state transitions for accounts and storage -->

This document describes the state transition operations of Cronos module, currently the only state is token mapping.

## Token Mapping

### Create/Update

There are several scenarios when the token mapping is updated:

- When auto-deployment is enabled and a new IBC or gravity token arrived.
- When auto-deployment is enabled and `MsgConvertVouchers` message executed with a new token.
- When a `TokenMappingChangeProposal` gov proposal is executed.
- When the admin account execute a `MsgUpdateTokenMapping` message.

When the mapping updated, the token migration between old and new contracts can be managed by contract developer, for example, the new contract could allow user to swap old token to new one.

#### Contract Migration

When token mapping get updated, the old contract still exists.

### Delete

There's no way to delete a token mapping currently.
