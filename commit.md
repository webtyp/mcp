This change refactors how tools are named, upgrading from simple strings (like `module.op`) to structured `router.Operation` definitions that encapsulate the module, entity, and action. It also updates `HarvestOps` to directly accept an `events.Publisher`, ensuring that write tools appropriately publish events when a mutation commits.

Additionally, this ensures tests align with `crud` v0.0.8, transitioning fixture setups from individual hooks to complete `crud.StoredEntity` models, fully integrated with definition properties and mocked `crud.Store` instances. `Caller.Call` and `Dispatch` methods have been adapted to the new strongly-typed operation parameter format, maintaining clarity on the client side.

Key updates:
* `router.Operation` defines operations (`Module`, `Entity`, `Action`).
* `HarvestOps` updated to integrate `events.Publisher`.
* Tests synchronized with the `crud` v0.0.8 requirement `crud.StoredEntity`.
* Comprehensive test fixes and new coverage scenarios for `HarvestOps` behavior.
