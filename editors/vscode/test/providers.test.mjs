import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { register } = require("../src/providers");

function harness() {
  const providers = {};
  const registrations = [];
  class Range {
    constructor(start, end) {
      this.start = start;
      this.end = end;
    }
  }
  class MarkdownString {
    value = "";
    appendText(text) {
      this.value += text;
      return this;
    }
  }
  class SnippetString {
    constructor(value) {
      this.value = value;
    }
  }
  class CompletionItem {
    constructor(label, kind) {
      this.label = label;
      this.kind = kind;
    }
  }
  class Hover {
    constructor(contents, range) {
      this.contents = contents;
      this.range = range;
    }
  }
  class Location {
    constructor(uri, range) {
      this.uri = uri;
      this.range = range;
    }
  }
  const languages = {};
  for (const name of ["CompletionItem", "Hover", "Definition"]) {
    languages[`register${name}Provider`] = (selector, provider, ...triggers) => {
      assert.deepEqual(selector, { language: "kartui" });
      providers[name] = provider;
      const registration = {
        disposed: false,
        dispose() {
          this.disposed = true;
        },
        triggers,
      };
      registrations.push(registration);
      return registration;
    };
  }
  const vscode = {
    languages,
    Range,
    MarkdownString,
    SnippetString,
    CompletionItem,
    Hover,
    Location,
    CompletionItemKind: { Value: 12, Property: 9, Class: 6, Variable: 5, Keyword: 13, EnumMember: 19 },
  };
  const context = { subscriptions: [] };
  register(vscode, context);
  const document = {
    version: 1,
    uri: "untitled:menu.kui",
    text: "",
    reads: 0,
    getText() {
      this.reads++;
      return this.text;
    },
    offsetAt(position) {
      return position;
    },
    positionAt(offset) {
      return offset;
    },
    update(source) {
      this.version++;
      this.text = source.replace("|", "");
      return source.indexOf("|");
    },
  };
  return { providers, registrations, context, document };
}
const token = { isCancellationRequested: false };

test("providers expose edit ranges/snippets, reuse document versions and invalidate after edits", () => {
  const { providers, document } = harness();
  let offset = document.update("<template><panel><checkbox onCh|/></panel></template>");
  const entries = providers.CompletionItem.provideCompletionItems(document, offset, token);
  const change = entries.find((entry) => entry.label === "onChange");
  assert.equal(change.insertText.value, "onChange={func(v bool) { ${1} }}");
  assert.equal(document.text.slice(change.range.start, change.range.end), "onCh");
  assert.ok(change.documentation.value.includes("func(bool)"));
  providers.CompletionItem.provideCompletionItems(document, offset, token);
  assert.equal(document.reads, 1);
  offset = document.update("<template><panel><input onCh|/></panel></template>");
  const input = providers.CompletionItem.provideCompletionItems(document, offset, token)[0];
  assert.equal(input.insertText.value, "onChange={func(v string) { ${1} }}");
  assert.equal(document.reads, 2);
});

test("hovers and definitions use the current buffer, safe text and UTF-16 positions", () => {
  const { providers, document } = harness();
  let offset = document.update(
    '<template><panel class="sc|reen"><label>😀</label></panel></template>\n<style>\n.screen\n  width: 80%\n</style>',
  );
  const definitions = providers.Definition.provideDefinition(document, offset, token);
  assert.equal(definitions.length, 1);
  assert.equal(definitions[0].uri, document.uri);
  assert.equal(document.text.slice(definitions[0].range.start, definitions[0].range.end), "screen");
  offset = document.update("<style>\n$space: [click](command:evil)\npanel\n  gap: $spa|ce\n</style>");
  const hover = providers.Hover.provideHover(document, offset, token);
  assert.equal(hover.contents.value, "[click](command:evil)");
  assert.equal(hover.contents.isTrusted, undefined);
});

test("cancellation avoids reading documents and provider registrations dispose", () => {
  const { providers, context, document, registrations } = harness();
  const cancelled = { isCancellationRequested: true };
  assert.deepEqual(providers.CompletionItem.provideCompletionItems(document, 0, cancelled), []);
  assert.equal(providers.Hover.provideHover(document, 0, cancelled), undefined);
  assert.deepEqual(providers.Definition.provideDefinition(document, 0, cancelled), []);
  assert.equal(document.reads, 0);
  assert.ok(registrations[0].triggers.includes(":"));
  assert.equal(context.subscriptions.length, 3);
  for (const disposable of context.subscriptions) disposable.dispose();
  assert.ok(registrations.every((registration) => registration.disposed));
});
