import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { createModel, contextAt, completions, hoverAt, definitionAt, MAX_SOURCE_LENGTH } = require("../src/model");

function input(source) {
  const offset = source.indexOf("|");
  assert.notEqual(offset, -1);
  return { model: createModel(source.replace("|", "")), offset };
}
function items(source) {
  const { model, offset } = input(source);
  return completions(model, offset);
}
function labels(source) {
  return items(source).map((item) => item.label);
}
const style = (text) => `<template><panel><label>Ready</label></panel></template>\n<style>\n${text}\n</style>`;
const markup = (text) => `<template>${text}</template>`;
function apply(source, label) {
  const entry = items(source).find((item) => item.label === label);
  assert.ok(entry, `missing completion ${label}`);
  const text = source.replace("|", "");
  return text.slice(0, entry.start) + entry.insert + text.slice(entry.end);
}

test("SFC openings, root panels, widgets and matching closing tags", () => {
  assert.deepEqual(labels("<scr|"), ["script"]);
  assert.deepEqual(labels("<!-- Menu -->\n<|"), ["script", "template", "style"]);
  assert.deepEqual(labels(markup("<|")), ["panel"]);
  assert.ok(labels(markup("<panel><ch|")).includes("checkbox"));
  assert.deepEqual(labels(markup("<panel><tabs><|")), ["tab"]);
  assert.deepEqual(labels(markup("<panel><tabs><tab></|")), ["tab", "tabs", "panel"]);
});

test("attributes depend on the widget and exclude existing attributes on either side", () => {
  const checkbox = labels(markup("<panel><checkbox ch| onChange={change}/></panel>"));
  assert.deepEqual(checkbox, ["checked"]);
  assert.ok(!labels(markup("<panel><input | value={name} onChange={change}/></panel>")).includes("value"));
  assert.ok(!labels(markup("<panel><slider |/></panel>")).includes("placeholder"));
  assert.deepEqual(labels(markup("<panel |><label>Text</label></panel>")), ["class", "modal", "onBack"]);
  assert.deepEqual(labels(markup("<panel><panel |/></panel>")), ["class", "tooltip"]);
  assert.ok(!labels(markup("<panel><tab |/></panel>")).includes("enabled"));
  assert.ok(apply(markup("<panel><slider mi|/></panel>"), "min").includes('min="${1:0}"'));
  assert.ok(!apply(markup('<panel mo|dal="true"/>'), "modal").includes('modal="${1'));
});

test("Go bodies, callback expressions, comments and literal labels have no markup suggestions", () => {
  const cases = [
    '<script lang="go" setup>func setup() { text := "<input |" }</script>\n<template><panel/></template>',
    markup('<panel><button onClick={func() { println("<input |") }}>Text</button></panel>'),
    markup("<panel><label>{`<input |`}</label></panel>"),
    markup("<panel><!-- <input | --></panel>"),
    markup("<panel><label>Text |</label></panel>"),
    style(".screen\n  // width: |"),
    style("/* .screen { width: | } */"),
  ];
  for (const source of cases) assert.deepEqual(labels(source), [], source);
});

test("Sass properties, values, short aliases and nested states are contextual", () => {
  assert.deepEqual(labels(style("panel\n  dir|")), ["direction"]);
  assert.deepEqual(labels(style("panel\n  direction: r|")), ["row"]);
  assert.deepEqual(labels(style("panel\n  align: s|")), ["start", "stretch"]);
  assert.ok(labels(style("panel\n  width: |")).includes("33.33%"));
  assert.deepEqual(labels(style(".action\n  &:ho|")), ["hover"]);
  assert.deepEqual(labels(style(".action\n  &:hover\n    |")), ["background", "color", "background-image"]);
  assert.deepEqual(labels(style("image\n  font-|")), []);
  assert.ok(!labels(style("input\n  |")).includes("icon"));
  assert.ok(!labels(style("tabs\n  |")).includes("padding"));
  assert.ok(apply(style("panel\n  wid|th: 80%"), "width").includes("width: 80%"));
  assert.ok(!apply(style("panel\n  width: 33.|33%"), "33.33%").includes("%%"));
});

test("legacy declarations do not offer authoring completion", () => {
  assert.deepEqual(labels("kartui Menu() { <panel><input pla|/></panel> }"), []);
  assert.deepEqual(labels("layout Frame { <panel><slot |/></panel> }"), []);
});

test("local classes and previously declared variables complete and navigate without script/comment leakage", () => {
  const source =
    '<script setup lang="go">func setup() { fake := ".fake" }</script>\n' +
    '<template><panel class="sc|reen"><label>Text</label></panel></template>\n<style>\n// .fake\n.screen\n  gap: 8\n</style>';
  assert.deepEqual(labels(source), ["screen"]);
  const { model, offset } = input(source);
  const definitions = definitionAt(model, offset);
  assert.equal(definitions.length, 1);
  assert.equal(model.text.slice(definitions[0].start, definitions[0].end), "screen");
  const variables = style("$space: 8px\npanel\n  gap: $spa|ce\n$later: 16px");
  assert.deepEqual(labels(variables), ["$space"]);
  const value = input(variables);
  assert.equal(definitionAt(value.model, value.offset)[0].value, "8px");
  assert.deepEqual(labels(style("panel\n  gap: $|\n$later: 16px")), []);
});

test("hover documents dimensions, callbacks and static style variables", () => {
  const dimension = input(style("panel\n  wi|dth: 80%"));
  assert.match(hoverAt(dimension.model, dimension.offset).detail, /0–100%.*parent content area/);
  const callback = input(markup("<panel><slider onCha|nge={change}/></panel>"));
  assert.match(hoverAt(callback.model, callback.offset).detail, /func\(int32\)/);
  const variable = input(style("$gap: 8px\npanel\n  gap: $ga|p"));
  assert.equal(hoverAt(variable.model, variable.offset).detail, "8px");
});

test("UTF-16 replacement offsets and bounded malformed editing buffers", () => {
  assert.ok(
    apply(markup("<panel><label>😀</label><input pla|/></panel>"), "placeholder").includes(
      "😀</label><input placeholder=",
    ),
  );
  assert.deepEqual(labels("<template><panel><button onClick={func() { /* |"), []);
  assert.deepEqual(labels("<template><panel><!-- <|"), []);
  const tooLarge = createModel(" ".repeat(MAX_SOURCE_LENGTH + 1));
  assert.deepEqual(completions(tooLarge, 4), []);
  assert.deepEqual(contextAt(tooLarge, 4), { type: "none" });
});

test("top-level comments, leaf controls and partial closing blocks stay contextual", () => {
  assert.deepEqual(labels("<!-- <|"), []);
  assert.deepEqual(labels("<!-- <inp| -->\n<template><panel/></template>"), []);
  assert.deepEqual(labels(markup("<panel><label><|</label></panel>")), []);
  assert.deepEqual(labels('<script setup lang="go">\nfunc setup() {}\n</scr|'), ["script"]);
  assert.deepEqual(labels("<style>\npanel\n  gap: 8\n</sty|"), ["style"]);
  assert.ok(apply(markup("<panel><list onCl|/></panel>"), "onClick").includes("func(id uint32)"));
});

test("large blank and deeply nested buffers stay within analysis bounds", () => {
  const blank = createModel(
    "<template>" + "\n".repeat(40000) + "</template>\n<style>" + "\n".repeat(40000) + "</style>",
  );
  assert.equal(blank.regions[0].tags.length, 0);
  const nested = createModel("<template>" + "<panel>".repeat(5000) + "</template>");
  assert.equal(nested.regions[0].tags.length, 4096);
  assert.ok(nested.regions[0].tags.every((tag) => tag.parents.length <= 64));
});

test("value widget callback snippets carry their exact Go argument types", () => {
  for (const [tag, type] of Object.entries({
    checkbox: "bool",
    input: "string",
    slider: "int32",
    combo: "uint32",
    tabs: "uint32",
  })) {
    const source = markup(`<panel><${tag} onCh|/></panel>`);
    assert.equal(items(source)[0].insert, `onChange={func(v ${type}) { \${1} }}`);
  }
});

test("multiline single-quoted attributes, conditional markup and CRLF style buffers work", () => {
  const source =
    "<!-- Header -->\r\n<template >\r\n<panel>\r\nif ready {\r\n<input\r\n value={name}\r\n pla|\r\n/>\r\n}\r\n</panel>\r\n</template >";
  assert.deepEqual(labels(source), ["placeholder"]);
  assert.deepEqual(labels("<template><panel modal='f|'/></template>"), ["false"]);
  assert.deepEqual(labels("<style lang='sass'>\r\npanel\r\n  justify: sp|\r\n</style >"), ["space-between"]);
  assert.deepEqual(labels(style("button:hover\n  wid|")), []);
});

test("state suggestions honor widget-specific restrictions", () => {
  assert.deepEqual(labels(style("input\n  &:|")), ["hover", "pressed", "disabled"]);
  assert.deepEqual(labels(style("label\n  &:|")), []);
  assert.deepEqual(labels(style("input\n  &:focus\n    |")), []);
});

test("closing-block lookalikes inside multiline Go strings and comments remain suppressed", () => {
  assert.deepEqual(labels('<script setup lang="go">\nfunc setup() { raw := `\n</scr|'), []);
  assert.deepEqual(labels('<script setup lang="go">\n/*\n</scr|'), []);
  assert.deepEqual(labels("<style>\n/*\n</sty|\n*/\n</style>"), []);
});
