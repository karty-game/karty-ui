# SFC-only authoring v1

The current KartUI compiler accepts `.kui` single-file components and layouts.
This is a breaking language and generator API change in the SDK 0.0.9 candidate.
Released SDK manifests, references and template snapshots remain immutable.

A component has one required `<template>` and optional Go `<script setup lang="go">`
and `<style>` blocks. The filename supplies its exported component name. Samples,
references and snippets put the template first, script second and style last.
The compiler still permits any block order. Layouts cannot have scripts.

Remove `kartui Name(...)`, `layout Name`, named or inline `setup` declarations,
`.ui` authoring discovery, brace-style authoring, and the deprecated `UIViewFile`
generator. Scripts use a final Go `func setup(...)` with named inputs; exported
scalar inputs no longer select a separate engine adapter. Every component uses
the ordinary client/package adapter, including components without a script.
Static level assets continue to reject setup and dynamic bindings.

Styles use indented rules. The optional `lang="sass"` attribute describes the same
syntax as plain `<style>`. Braces, semicolons or tabs warn and ignore the block;
invalid properties and values warn and ignore individual declarations. Structural
indentation, nesting and source limits remain errors. The internal resolved rule
representation is an implementation detail, not a second authoring format.

The CLI consumes the client/package generators and `.kui` discovery. Candidate
SDK templates move to 0.0.2; released template snapshots are retained. No wire,
presentation schema, callback or runtime API change is needed for this cleanup.

Migration means putting markup in `<template>`, imports/types and setup logic in
`<script setup lang="go">`, converting rules to indentation, and renaming files
so their names match their exported component or layout names.
