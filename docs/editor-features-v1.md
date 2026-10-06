# KartUI editor features v1

Status: local VS Code extension 0.1.21. This adds editor services without changing
KartUI source syntax, compiler APIs, serialized schemas, SDK pins or host behavior.

The first increment for the semantic editor review item supplies context-aware
completion, hover help and local style navigation. The extension registers
completion, hover and definition providers directly using the supported
[VS Code language feature APIs](https://code.visualstudio.com/api/language-extensions/programmatic-language-features).
It needs no language-server process or extra runtime dependency.

Completion recognizes SFC block openings, markup tags, matching closing tags,
widget attributes, literal boolean values, style properties/aliases, suggested
values, local classes and previously declared Sass variables. Attribute snippets
use the widget's callback argument type. Suggestions exclude attributes already
present on either side of the cursor and restrict style properties when a target
widget or state rule is known. Go script and binding expressions, comments and
plain text bodies receive no KartUI suggestions. Indented styles
are supported. Legacy component/layout declarations receive no completion.
`.kui.tmpl` files receive best-effort help for the underlying KartUI text, not
Go-template language services.

Hover help documents property units, ranges and dependencies and attribute
bindings. Local class and Sass variable references navigate to definitions in
the same document, including unsaved documents. Classes with multiple rules may
have several definition targets. A variable reference selects its last preceding
definition. Qualified layout overrides need cross-file analysis and are not
resolved by this increment.

The source model tolerates incomplete edits and uses UTF-16 offsets for VS Code
edit ranges. It is cached by document identity/version. Source analysis stops
above 256 Ki UTF-16 code units; markup analysis is bounded at 4096 tags and a
64-entry parent stack. These are editor resource limits; compiler asset/element
bounds remain stricter. Cancelled requests avoid source reads. Authored variable
text is displayed as untrusted plain text, not executable Markdown links. There
are no network requests, subprocesses or workspace writes. Only current-buffer
text is read, supporting virtual and untrusted workspaces. Desktop/remote VS Code
extension hosts are supported; no browser extension bundle is provided.

The property catalog is handwritten and tracked. Go tests compare its entire
property set, aliases, value examples and advertised target widgets with the
compiler's style parser and schema validation. Node tests exercise markup/style
contexts, nested Go quoting/comments, partial attributes, replacement ranges,
multiline blocks, Unicode offsets, local references, provider registration,
cache invalidation, cancellation and bounded malformed buffers. Existing
tokenization and compiling snippet/reference tests remain in place.

Next increments are compiler diagnostics for unsaved files with project
layouts/themes, cross-file component/layout completion and navigation, a
formatter that preserves Go and Sass semantics, and embedded Go/gopls integration
with source maps. This extension does not currently provide those features,
Go type checking, rename or automatic tag closing. The compiler remains the
authority on valid source; editor suggestions do not perform compilation.

Validation and packaging outcomes are recorded in
[editor features validation](editor-features-validation.md).
