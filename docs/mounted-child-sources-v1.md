# Mounted child sources, version 1

Decision: add the optional `source={expression}` attribute to child components
for the SDK 0.0.13 / API 0.0.11 candidate. The expression must have type
`engine.UISource`. Generated adapters call `UI.ChildSource(slot, key, source)`;
the named child component still supplies the typed props, setup and callbacks.
The selected template must preserve its element IDs, slots and action bindings.

Omitting `source` retains the ordinary `UI.Child` adapter and does not introduce
a dependency on `UISource` or `ChildSource`. A zero handle selects a global
template; a nonzero handle selects a ready level's template. Request the owning
pack before mounting. A changed handle/name replaces the keyed child; releasing
the pack removes its descendants. The engine owns readiness and lifecycle checks.

This is an additive language and generator API change (`Binding.Source` and
`Component.SourceFactory`). It changes neither the UI schema nor its wire format.
Programs using the attribute require the matching SDK APIs; releasing KartUI
does not establish that the SDK candidate is publicly available. Existing
programs without the attribute keep their SDK requirements.

The pinned [language reference](0.0.1/kartui.md#mounted-child-templates-sdk-0013-candidate)
documents authoring. Compiler tests cover extraction and rejection of literal,
duplicate and unknown attributes. Adapter tests compile and execute generated
same-package and importable clients against a public-contract fixture. Editor
tests cover completion, hover, duplicate suppression and Go-expression boundaries.
These checks do not execute the actual SDK, WASM or browser graphics.
