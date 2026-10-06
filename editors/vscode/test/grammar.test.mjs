import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import textmate from "vscode-textmate";
import oniguruma from "vscode-oniguruma";

const require = createRequire(import.meta.url);
const wasm = await readFile(require.resolve("vscode-oniguruma/release/onig.wasm"));
await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
const source = await readFile(new URL("../syntaxes/kartui.tmLanguage.json", import.meta.url), "utf8");
const registry = new textmate.Registry({
  onigLib: Promise.resolve({
    createOnigScanner: (p) => new oniguruma.OnigScanner(p),
    createOnigString: (s) => new oniguruma.OnigString(s),
  }),
  loadGrammar: async (scope) => (scope === "source.kartui" ? textmate.parseRawGrammar(source, "kartui.json") : null),
});
const grammar = await registry.loadGrammar("source.kartui");

function tokenize(source) {
  let state = textmate.INITIAL;
  const result = [];
  for (const line of source.split("\n")) {
    const next = grammar.tokenizeLine(line, state);
    result.push(...next.tokens.map((t) => ({ text: line.slice(t.startIndex, t.endIndex), scopes: t.scopes })));
    state = next.ruleStack;
  }
  return result;
}
function has(tokens, text, scope) {
  assert.ok(
    tokens.some((t) => t.text === text && t.scopes.includes(scope)),
    `${JSON.stringify(text)} missing ${scope}`,
  );
}

test("actual demo declaration, parameters, markup and bindings", async () => {
  const demo = await readFile(new URL("./fixtures/menu.kui", import.meta.url), "utf8");
  const tokens = tokenize(demo);
  has(tokens, "AppProps", "variable.other.go");
  has(tokens, "setup", "meta.embedded.script.go");
  has(tokens, "MainMenuLayout", "entity.name.tag.html");
  has(tokens, "onClick", "entity.other.attribute-name.html");
  has(tokens, "icon", "support.type.property-name.css");
  has(tokens, "choose", "meta.embedded.expression.go");
});

test("SFC Go script, template and style blocks use embedded language scopes", () => {
  const tokens = tokenize(
    '<script setup lang="go">\nfunc setup(args MenuProps) {\nchoose := func() { args.Select() }\n}\n</script>\n' +
      "<template><panel><button onClick={ choose }>Play</button></panel></template>\n" +
      "<style>\n.menu\n  color: theme.colors.text\n</style>",
  );
  has(tokens, "func", "meta.embedded.script.go");
  has(tokens, "setup", "meta.embedded.script.go");
  has(tokens, "MenuProps", "meta.embedded.script.go");
  has(tokens, "panel", "entity.name.tag.html");
  has(tokens, "onClick", "entity.other.attribute-name.html");
  has(tokens, ".menu", "entity.other.attribute-name.class.css");
});

test("shared SFC compiler sample stays highlighted in all three blocks", async () => {
  const source = await readFile(new URL("../../../samples/sfc-demo/ui/views/menu.kui", import.meta.url), "utf8");
  const tokens = tokenize(source);
  has(tokens, "func", "meta.embedded.script.go");
  has(tokens, "MenuProps", "meta.embedded.script.go");
  has(tokens, "MainMenu", "entity.name.tag.html");
  has(tokens, "onClick", "entity.other.attribute-name.html");
  has(tokens, "&:hover", "entity.other.attribute-name.class.css");
});

test("nested Go braces, comments, escaped strings and raw strings do not swallow tags", () => {
  const tokens = tokenize(
    '<template>\n<panel modal="true">\n' +
      "<button onClick={ func() { /* } > */ if true { Click() } } }>Click</button>\n" +
      '<label>{ "quoted \\\" } >" + `raw }\n >` }</label>\n' +
      "<!-- { <fake> } -->\n<label>Done</label>\n</panel>\n</template>\n" +
      "<style>\n.next\n  gap: 8\n</style>",
  );
  has(tokens, "button", "entity.name.tag.html");
  has(tokens, "label", "entity.name.tag.html");
  has(tokens, ".next", "entity.other.attribute-name.class.css");
  assert.ok(tokens.some((t) => t.scopes.includes("string.quoted.raw.go")));
  assert.ok(!tokens.some((t) => t.text === "fake" && t.scopes.includes("entity.name.tag.html")));
  const done = tokens.find((t) => t.text === "Done");
  assert.ok(done && !done.scopes.includes("meta.embedded.expression.go"));
});

test("manifest retains language and snippet contributions alongside local providers", async () => {
  const manifest = JSON.parse(await readFile(new URL("../package.json", import.meta.url)));
  assert.equal(manifest.main, "./src/extension.js");
  assert.equal(manifest.browser, undefined);
  assert.equal(manifest.contributes.languages[0].id, "kartui");
  assert.deepEqual(manifest.contributes.languages[0].extensions, [".kui", ".kui.tmpl"]);
  const snippets = JSON.parse(await readFile(new URL("../snippets/kartui.json", import.meta.url)));
  assert.ok(snippets.Component.body.includes('<script setup lang="go">'));
  assert.equal(snippets["Legacy component"], undefined);
  for (const snippet of Object.values(snippets)) {
    const template = snippet.body.indexOf("<template>"),
      script = snippet.body.indexOf('<script setup lang="go">');
    if (template >= 0 && script >= 0) assert.ok(template < script);
  }
  JSON.parse(await readFile(new URL("../language-configuration.json", import.meta.url)));
});

test("keyed component loop keeps markup highlighted", async () => {
  const source = await readFile(new URL("./fixtures/inventory.kui", import.meta.url), "utf8");
  const tokens = tokenize(source);
  has(tokens, "for", "keyword.control.go");
  has(tokens, "ItemRow", "entity.name.tag.html");
  has(tokens, "key", "entity.other.attribute-name.html");
  has(tokens, "props", "entity.other.attribute-name.html");
  has(tokens, "onBack", "entity.other.attribute-name.html");
});

test("style blocks highlight selectors, properties and theme tokens", () => {
  const tokens = tokenize(
    '<template><panel class="screen"/></template>\n<style>\n.screen:hover\n  background-image: theme.images.panel\n  image-fit: contain\n  direction: row\n  align: center\n  justify: space-between\n  overflow: scroll\n  text-align: left\n  font-family: display\n</style>',
  );
  has(tokens, ".screen:hover", "entity.other.attribute-name.class.css");
  has(tokens, "background-image", "support.type.property-name.css");
  has(tokens, "theme.images.panel", "variable.other.constant.css");
  has(tokens, "direction", "support.type.property-name.css");
  has(tokens, "row", "support.constant.property-value.css");
  has(tokens, "center", "support.constant.property-value.css");
  has(tokens, "space-between", "support.constant.property-value.css");
  has(tokens, "scroll", "support.constant.property-value.css");
  has(tokens, "text-align", "support.type.property-name.css");
  has(tokens, "left", "support.constant.property-value.css");
  has(tokens, "font-family", "support.type.property-name.css");
  has(tokens, "display", "support.constant.property-value.css");
  has(tokens, "contain", "support.constant.property-value.css");
});

test("layout declarations and slots are highlighted", async () => {
  const source = await readFile(new URL("./fixtures/window-layout.kui", import.meta.url), "utf8");
  const tokens = tokenize(source);
  has(tokens, "slot", "entity.name.tag.html");
  has(tokens, "@media", "keyword.control.at-rule.media.css");
  has(tokens, "max-width", "support.type.property-name.media.css");
});

test("conditional markup keeps Go conditions and both branches highlighted", () => {
  const tokens = tokenize(
    "<template><panel>\nif len(items) == 0 {\n<label>Empty</label>\n} else {\n<button>Use</button>\n}\n</panel></template>",
  );
  has(tokens, "if", "keyword.control.go");
  has(tokens, "else", "keyword.control.go");
  has(tokens, "label", "entity.name.tag.html");
  has(tokens, "button", "entity.name.tag.html");
});

test("typed widgets and tooltip attributes are highlighted", () => {
  const tokens = tokenize(
    '<template><panel><checkbox checked={enabled} onChange={change} tooltip="Audio"/><input value={name} placeholder="Name"/><slider min="0" max="100"/><combo rows={choices} selected={selection}/><tabs selected={active}><tab title="General"/></tabs></panel></template>',
  );
  for (const tag of ["checkbox", "input", "slider", "combo", "tabs", "tab"]) has(tokens, tag, "entity.name.tag.html");
  for (const attribute of [
    "checked",
    "onChange",
    "tooltip",
    "value",
    "placeholder",
    "min",
    "max",
    "rows",
    "selected",
    "title",
  ])
    has(tokens, attribute, "entity.other.attribute-name.html");
});

test("indented styles highlight variables, states, units and leave the next block intact", () => {
  for (const opening of ["<style>", '<style lang="sass">']) {
    const tokens = tokenize(
      opening +
        "\n$space: theme.spacing.gap\n.primary\n  width: 33.33%\n  height: 44px\n  align: stretch\n  gap: $space\n  &:hover\n    background: theme.colors.primary-hover\n  // local comment\n</style>\n<template><panel/></template>",
    );
    has(tokens, "$space", "variable.other.sass");
    has(tokens, ".primary", "entity.other.attribute-name.class.css");
    has(tokens, "&:hover", "entity.other.attribute-name.class.css");
    has(tokens, "width", "support.type.property-name.css");
    has(tokens, "align", "support.type.property-name.css");
    has(tokens, "33.33%", "constant.numeric.css");
    has(tokens, "44px", "constant.numeric.css");
    has(tokens, "panel", "entity.name.tag.html");
  }
});

test("editor includes compact SFC, nesting and percentage sizing snippets", async () => {
  const snippets = JSON.parse(await readFile(new URL("../snippets/kartui.json", import.meta.url)));
  assert.ok(snippets["SFC component"].body.includes("<style>"));
  assert.ok(snippets["Indented styles"].body.some((line) => line.includes("width:")));
  assert.ok(snippets["Nested widget state"].body[0].startsWith("&:"));
  assert.ok(snippets["Parent-relative size"].body.some((line) => line.includes("100%")));
});

test("indented selectors and states indent their declarations", async () => {
  const config = JSON.parse(await readFile(new URL("../language-configuration.json", import.meta.url)));
  const increase = new RegExp(config.indentationRules.increaseIndentPattern);
  for (const line of [".screen", "  &:hover", "button", "@media (max-width: 480)", ".primary // comment"]) {
    assert.ok(increase.test(line), `${line} should indent the next line`);
  }
  for (const line of ["  width: 80%", "  direction: row", "$space: 8px"]) {
    assert.ok(!increase.test(line), `${line} should preserve declaration indentation`);
  }
});

test("ordinary SFC edits and qualified layout overrides remain highlighted", () => {
  for (const opening of ['<script lang="go" setup>', "<script setup lang='go' >", '<script\n lang="go"\n setup\n>']) {
    const tokens = tokenize(
      "<!-- Screen -->\n" +
        opening +
        "\nfunc setup() { volume := int32(50) }\n</script >\n<template ><panel><label>Ready</label></panel></template >\n<style lang='sass' >\nFrame.content\n  gap: 12px\n</style >",
    );
    has(tokens, "func", "meta.embedded.script.go");
    has(tokens, "int32", "storage.type.go");
    has(tokens, "label", "entity.name.tag.html");
    has(tokens, "Frame.content", "entity.other.attribute-name.class.css");
  }
});

test("widgets and settings examples are available as authoring snippets", async () => {
  const snippets = JSON.parse(await readFile(new URL("../snippets/kartui.json", import.meta.url)));
  for (const name of [
    "Checkbox",
    "Text input",
    "Slider",
    "Combo",
    "Tabs",
    "Tooltip",
    "Settings screen",
    "Layout class override",
  ])
    assert.ok(snippets[name]);
  assert.ok(snippets["Settings screen"].body.includes('<script setup lang="go">'));
  const config = JSON.parse(await readFile(new URL("../language-configuration.json", import.meta.url)));
  assert.ok(new RegExp(config.indentationRules.increaseIndentPattern).test("Frame.content"));
});
